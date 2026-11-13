package watcher

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recorder struct {
	mu       sync.Mutex
	triggers []Trigger
	running  int32
	maxPar   int32
}

func (r *recorder) fn(work time.Duration) func(Trigger) {
	return func(t Trigger) {
		n := atomic.AddInt32(&r.running, 1)
		for {
			m := atomic.LoadInt32(&r.maxPar)
			if n <= m || atomic.CompareAndSwapInt32(&r.maxPar, m, n) {
				break
			}
		}
		time.Sleep(work)
		atomic.AddInt32(&r.running, -1)
		r.mu.Lock()
		r.triggers = append(r.triggers, t)
		r.mu.Unlock()
	}
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.triggers)
}

// A1 : un evenement arrive PENDANT un cycle => exactement un cycle de plus, apres
// la fin du premier ; jamais deux cycles en meme temps.
func TestScheduler_UnCycleALaFois_EtExactementUnDePlus(t *testing.T) {
	s := newScheduler(50*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(400*time.Millisecond))

	s.Event("a")
	time.Sleep(150 * time.Millisecond) // le cycle 1 est en cours (50 ms de fusion + 400 ms de travail)
	if got := atomic.LoadInt32(&r.running); got != 1 {
		t.Fatalf("cycle 1 devrait tourner, running=%d", got)
	}
	// 5 evenements pendant le cycle : ils doivent se fusionner en UN cycle
	for i := 0; i < 5; i++ {
		s.Event("b")
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(1500 * time.Millisecond)
	if n := r.count(); n != 2 {
		t.Fatalf("attendu exactement 2 cycles (1 + 1 de plus), obtenu %d", n)
	}
	if r.maxPar != 1 {
		t.Fatalf("deux cycles en parallele : max=%d", r.maxPar)
	}
}

// A2 : sous un flux continu d'evenements (plus rapproches que la fenetre), un
// cycle part quand meme au plus tard maxDelay apres le premier evenement.
func TestScheduler_DelaiMaximal_PasDeFamine(t *testing.T) {
	s := newScheduler(200*time.Millisecond, 600*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))

	start := time.Now()
	stop := time.After(1500 * time.Millisecond)
	tick := time.NewTicker(50 * time.Millisecond) // 4x plus rapide que la fenetre
	defer tick.Stop()
loop:
	for {
		select {
		case <-tick.C:
			s.Event("flux")
		case <-stop:
			break loop
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.triggers) == 0 {
		t.Fatal("aucun cycle sous flux continu : famine (fenetre sans borne)")
	}
	first := r.triggers[0]
	if lag := first.ScheduledAt.Sub(first.FirstEventAt); lag > 700*time.Millisecond {
		t.Fatalf("premier cycle trop tard : %s apres le premier evenement (max 600ms)", lag)
	}
	if first.ScheduledAt.Sub(start) > 800*time.Millisecond {
		t.Fatalf("premier cycle a %s, attendu < 800 ms", first.ScheduledAt.Sub(start))
	}
	t.Logf("%d cycles en 1,5 s de flux continu ; premier a %s", len(r.triggers), first.ScheduledAt.Sub(start).Round(time.Millisecond))
}

// A2 : sans flux, la fenetre de fusion garde son role (un seul cycle pour une rafale courte).
func TestScheduler_FenetreDeFusion(t *testing.T) {
	s := newScheduler(150*time.Millisecond, 2*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))
	for i := 0; i < 4; i++ {
		s.Event("x")
		time.Sleep(60 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	if n := r.count(); n != 1 {
		t.Fatalf("rafale courte : attendu 1 cycle, obtenu %d", n)
	}
}

// A3 : une seule minuterie d'echeance ; la replanification remplace la precedente.
func TestScheduler_UneSeuleEcheance(t *testing.T) {
	s := newScheduler(50*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))

	s.SetDeadline(time.Now().Add(150 * time.Millisecond))
	s.SetDeadline(time.Now().Add(400 * time.Millisecond)) // remplace la premiere
	time.Sleep(250 * time.Millisecond)
	if n := r.count(); n != 0 {
		t.Fatalf("la 1re echeance aurait du etre remplacee, %d cycle(s) deja", n)
	}
	time.Sleep(400 * time.Millisecond)
	if n := r.count(); n != 1 {
		t.Fatalf("attendu 1 reveil, obtenu %d", n)
	}
	if c := r.triggers[0].Cause; c != CauseDeadline {
		t.Fatalf("cause attendue %q, obtenue %q", CauseDeadline, c)
	}
	if !s.Deadline().IsZero() {
		t.Fatal("echeance consommee : Deadline() devrait etre zero")
	}
	// zero = aucune echeance
	s.SetDeadline(time.Now().Add(100 * time.Millisecond))
	s.SetDeadline(time.Time{})
	time.Sleep(300 * time.Millisecond)
	if n := r.count(); n != 1 {
		t.Fatalf("echeance annulee mais un reveil a eu lieu (%d)", n)
	}
}

// Un evenement et une echeance prets en meme temps : un seul cycle, cause evenement
// (il juge aussi), pas de reveil superflu derriere.
func TestScheduler_EvenementCouvreEcheance(t *testing.T) {
	s := newScheduler(100*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))
	s.SetDeadline(time.Now().Add(100 * time.Millisecond))
	s.Event("x")
	time.Sleep(500 * time.Millisecond)
	if n := r.count(); n != 1 {
		t.Fatalf("attendu 1 cycle, obtenu %d", n)
	}
	if r.triggers[0].Cause != CauseEvent {
		t.Fatalf("cause attendue evenement, obtenue %s", r.triggers[0].Cause)
	}
}

// R1 : une reprise (attente donnee) donne UN cycle de cause "reprise", une seule fois.
func TestScheduler_Reprise_UneSeuleFois(t *testing.T) {
	s := newScheduler(50*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))
	s.Retry(100 * time.Millisecond)
	time.Sleep(600 * time.Millisecond)
	if r.count() != 1 || r.triggers[0].Cause != CauseRetry {
		t.Fatalf("attendu 1 cycle 'reprise', obtenu %+v", r.triggers)
	}
}

// R1 : un cycle d'evenement couvre la reprise en attente (pas de cycle de trop) ;
// CancelRetry l'annule.
func TestScheduler_EvenementCouvreReprise(t *testing.T) {
	s := newScheduler(20*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &recorder{}
	go s.Run(ctx, r.fn(0))
	s.Retry(300 * time.Millisecond)
	s.Event("x")
	time.Sleep(700 * time.Millisecond)
	if r.count() != 1 || r.triggers[0].Cause != CauseEvent {
		t.Fatalf("attendu 1 cycle 'evenement', obtenu %+v", r.triggers)
	}
	s.Retry(100 * time.Millisecond)
	s.CancelRetry()
	time.Sleep(400 * time.Millisecond)
	if r.count() != 1 {
		t.Fatalf("la reprise annulee a tout de meme tourne : %+v", r.triggers)
	}
}

// R1 : si le cycle qui vient de partir echoue, l'echeance connue n'est pas perdue
// et une reprise est programmee (attente 5 s, puis croissante, plafond 60 s).
func TestAfterCycle_EchecConserveEcheanceEtReprogrammeReprise(t *testing.T) {
	w := &Watcher{sched: newScheduler(time.Second, 5*time.Second)}
	dl := time.Now().Add(time.Hour)
	w.deadline = dl
	w.afterCycle(nil, context.DeadlineExceeded)
	if !w.sched.Deadline().Equal(dl) {
		t.Fatalf("echeance perdue : %v", w.sched.Deadline())
	}
	if w.sched.rtTimer == nil || w.failures != 1 {
		t.Fatalf("reprise non programmee (failures=%d)", w.failures)
	}
	for i := 0; i < 10; i++ {
		w.afterCycle(nil, context.DeadlineExceeded)
	}
	if w.failures != 11 {
		t.Fatalf("failures=%d", w.failures)
	}
	w.afterCycle(nil, nil) // un succes arrete tout
	if w.failures != 0 || w.sched.rtTimer != nil {
		t.Fatalf("la reprise doit s'arreter au premier succes")
	}
}
