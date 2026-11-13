package watcher

import (
	"context"
	"sync"
	"time"
)

// Cause d'un cycle (journalisee, A7).
const (
	CauseEvent    = "evenement" // au moins un evenement Watch a ete recu
	CauseDeadline = "echeance"  // reveil programme (A3) : jugement seul, sans lecture du cluster
	CauseRetry    = "reprise"   // reprise apres un cycle en echec (attente croissante bornee)
)

// Trigger : ce qui a declenche un cycle.
type Trigger struct {
	Cause string
	// Cluster : cluster du premier evenement en attente (cause "evenement").
	Cluster string
	// FirstEventAt : instant du PREMIER evenement du lot fusionne (zero pour une echeance).
	FirstEventAt time.Time
	// ScheduledAt : instant ou le cycle a ete declare pret (fin de la fusion
	// d'evenements, ou date d'echeance atteinte).
	ScheduledAt time.Time
	// Objects : les premiers objets (cluster, type, nom, operation, champs
	// modifies) du lot fusionne ; Events : nombre total d'evenements du lot.
	Objects []string
	Events  int
}

// scheduler : UN executeur de cycles, UNE cle.
//
// Pourquoi pas un workqueue controller-runtime a cle unique : il apporterait la
// meme garantie "un evenement pendant un cycle = exactement un cycle de plus",
// mais ni fenetre de fusion avec delai maximal, ni minuterie d'echeance, ni cause
// du declenchement. Ces trois points sont necessaires (A2, A3, A7). Les
// evenements Watch restent fournis par le controller multi-cluster (Reconcile
// qui appelle Event) ; seule l'execution des cycles est ici.
//
// Garanties :
//   - un seul cycle a la fois (une seule goroutine execute fn) ;
//   - un evenement arrive PENDANT un cycle provoque exactement un cycle de plus,
//     apres la fin du premier (les declenchements prets sont fusionnes) ;
//   - fenetre de fusion glissante de `debounce`, bornee par `maxDelay` depuis
//     le premier evenement en attente (aucune famine sous flux continu) ;
//   - une seule minuterie d'echeance (SetDeadline la remplace).
type scheduler struct {
	debounce time.Duration
	maxDelay time.Duration

	mu sync.Mutex
	// evenements en cours de fusion
	evPending bool
	evFirst   time.Time
	evSrc     string
	evObjs    []string
	evCount   int
	evTimer   *time.Timer
	// declenchements prets (en attente de l'executeur)
	ready   *Trigger
	dlReady *Trigger
	// minuterie d'echeance : une seule
	dlTimer *time.Timer
	dlAt    time.Time
	// dlGen : numero de l'echeance courante. Time.Stop ne peut pas rattraper une
	// minuterie DEJA sonnee dont le callback attend le verrou : le callback
	// compare donc son numero a dlGen et s'efface s'il est perime.
	dlGen uint64

	// minuterie de reprise (cycle en echec) : une seule, annulee des qu'un cycle
	// complet part (il couvre la reprise : s'il echoue, il en reprogramme une).
	rtTimer *time.Timer
	rtGen   uint64

	kick chan struct{} // capacite 1 : les signaux se fusionnent
}

func newScheduler(debounce, maxDelay time.Duration) *scheduler {
	if debounce <= 0 {
		debounce = 2 * time.Second
	}
	if maxDelay < debounce {
		maxDelay = debounce
	}
	return &scheduler{debounce: debounce, maxDelay: maxDelay, kick: make(chan struct{}, 1)}
}

func (s *scheduler) signal() {
	select {
	case s.kick <- struct{}{}:
	default: // un signal est deja en attente : ils se fusionnent
	}
}

// maxObjects : nombre maximal d'objets distincts gardes par lot pour le journal.
const maxObjects = 8

// noteObject : decrit l'objet d'un evenement (journal). Appele par le handler
// juste AVANT que le controller ne rappelle Event : la note est donc rattachee au
// lot en cours de fusion. Ne declenche rien.
func (s *scheduler) noteObject(desc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if desc == "" || len(s.evObjs) >= maxObjects {
		return
	}
	for _, o := range s.evObjs {
		if o == desc {
			return
		}
	}
	s.evObjs = append(s.evObjs, desc)
}

// Event : un evenement Watch. Ne bloque jamais.
func (s *scheduler) Event(src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.evPending {
		s.evPending, s.evFirst, s.evSrc = true, now, src
	}
	s.evCount++
	// fenetre glissante, mais jamais au-dela de evFirst + maxDelay
	wait := s.debounce
	if limit := s.evFirst.Add(s.maxDelay).Sub(now); limit < wait {
		wait = limit
	}
	if wait < 0 {
		wait = 0
	}
	if s.evTimer != nil {
		s.evTimer.Stop()
	}
	s.evTimer = time.AfterFunc(wait, s.eventWindowClosed)
}

func (s *scheduler) eventWindowClosed() {
	s.mu.Lock()
	if !s.evPending {
		s.mu.Unlock()
		return
	}
	t := Trigger{Cause: CauseEvent, Cluster: s.evSrc, FirstEventAt: s.evFirst, ScheduledAt: time.Now(),
		Objects: s.evObjs, Events: s.evCount}
	s.evObjs, s.evCount = nil, 0
	if s.ready != nil {
		// un lot deja pret et pas encore execute : on fusionne
		if s.ready.FirstEventAt.Before(t.FirstEventAt) {
			t.FirstEventAt, t.Cluster = s.ready.FirstEventAt, s.ready.Cluster
		}
		t.Events += s.ready.Events
		t.Objects = append(append([]string{}, s.ready.Objects...), t.Objects...)
		if len(t.Objects) > maxObjects {
			t.Objects = t.Objects[:maxObjects]
		}
	}
	s.ready = &t
	s.evPending = false
	s.mu.Unlock()
	s.signal()
}

// SetDeadline programme UN reveil a la date `at` (zero = aucun). Remplace toute
// echeance precedente, y compris un reveil deja pret mais pas encore execute :
// l'appelant vient de juger, son echeance est la plus recente.
func (s *scheduler) SetDeadline(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dlTimer != nil {
		s.dlTimer.Stop()
		s.dlTimer = nil
	}
	s.dlReady = nil
	s.dlAt = at
	s.dlGen++
	if at.IsZero() {
		return
	}
	d := time.Until(at)
	if d < 0 {
		d = 0
	}
	gen := s.dlGen
	s.dlTimer = time.AfterFunc(d, func() { s.deadlineReached(gen) })
}

// Retry : programme UNE reprise dans `after` (cycle complet, cause "reprise").
// Remplace la precedente. Ce n'est pas une periode : l'appelant ne la
// reprogramme que si le cycle echoue, et l'arrete au premier succes (CancelRetry).
func (s *scheduler) Retry(after time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rtTimer != nil {
		s.rtTimer.Stop()
	}
	s.rtGen++
	gen := s.rtGen
	s.rtTimer = time.AfterFunc(after, func() { s.retryReached(gen) })
}

// CancelRetry : annule la reprise en attente.
func (s *scheduler) CancelRetry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rtTimer != nil {
		s.rtTimer.Stop()
		s.rtTimer = nil
	}
	s.rtGen++
}

func (s *scheduler) retryReached(gen uint64) {
	s.mu.Lock()
	if gen != s.rtGen {
		s.mu.Unlock()
		return
	}
	s.rtTimer = nil
	if s.ready == nil && !s.evPending { // sinon un cycle d'evenement va partir et couvre la reprise
		s.ready = &Trigger{Cause: CauseRetry, ScheduledAt: time.Now()}
	}
	s.mu.Unlock()
	s.signal()
}

// Deadline : echeance actuellement programmee (zero = aucune).
func (s *scheduler) Deadline() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dlAt
}

func (s *scheduler) deadlineReached(gen uint64) {
	s.mu.Lock()
	if gen != s.dlGen { // echeance remplacee ou couverte par un cycle d'evenement
		s.mu.Unlock()
		return
	}
	s.dlAt = time.Time{}
	if s.evPending || s.ready != nil {
		// Un cycle d'evenement est deja en cours de fusion ou pret : il jugera
		// aussi, et reprogrammera l'echeance. Un reveil ici serait un jugement
		// de trop. (Pas de famine : evenements bornes par maxDelay.)
		s.mu.Unlock()
		return
	}
	s.dlReady = &Trigger{Cause: CauseDeadline, ScheduledAt: time.Now()}
	s.mu.Unlock()
	s.signal()
}

// take : le prochain declenchement a executer. Un cycle d'evenement couvre une
// echeance (il juge aussi) : on ne garde alors que lui.
func (s *scheduler) take() (Trigger, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready != nil {
		t := *s.ready
		s.ready, s.dlReady = nil, nil
		// cycle complet : il couvre aussi la reprise en attente
		if s.rtTimer != nil {
			s.rtTimer.Stop()
			s.rtTimer = nil
		}
		s.rtGen++
		// Ce cycle juge : il couvre l'echeance en attente, et l'appelant en
		// reprogramme une nouvelle des qu'il a juge (SetDeadline). Sans cela, une
		// echeance qui sonne entre le depart et la fin de ce cycle provoquerait
		// un jugement de trop.
		if s.dlTimer != nil {
			s.dlTimer.Stop()
			s.dlTimer = nil
		}
		s.dlAt = time.Time{}
		s.dlGen++ // invalide un callback deja sonne
		return t, true
	}
	if s.dlReady != nil {
		t := *s.dlReady
		s.dlReady = nil
		return t, true
	}
	return Trigger{}, false
}

// Run : l'executeur unique. Bloque jusqu'a l'annulation de ctx. fn n'est jamais
// appelee deux fois en meme temps.
func (s *scheduler) Run(ctx context.Context, fn func(Trigger)) {
	for {
		select {
		case <-ctx.Done():
			s.stop()
			return
		case <-s.kick:
		}
		for {
			t, ok := s.take()
			if !ok {
				break
			}
			fn(t)
			if ctx.Err() != nil {
				s.stop()
				return
			}
		}
	}
}

func (s *scheduler) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.evTimer != nil {
		s.evTimer.Stop()
	}
	if s.dlTimer != nil {
		s.dlTimer.Stop()
	}
	if s.rtTimer != nil {
		s.rtTimer.Stop()
	}
}
