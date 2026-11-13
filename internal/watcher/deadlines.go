package watcher

import (
	"sync"
	"time"

	"github.com/orange/sylvatwin-operator/internal/judge"
)

// deadlineState : les echeances connues apres le dernier jugement, et la regle
// d'armement de l'UNIQUE minuterie de l'ordonnanceur.
//
// Echeance d'horizon : fixe (now + delai SPARQL).
// Echeance de fraicheur d'un cluster A TEMOIN : dernier evenement de Lease recu
// + fenetre (SPARQL) + 1 s. Chaque Lease recue la repousse (rearm), SANS cycle
// ni jugement : le reveil ne sonne que si le temoin se tait plus longtemps que
// son seuil. Pour un cluster sans temoin : maintenant + delai SPARQL (fixe).
// La decision "inconnu" reste dans rules/verdict ; ici on ne decide que QUAND
// re-juger.
type deadlineState struct {
	mu     sync.Mutex
	active bool // faux pendant un cycle : il reprogrammera lui-meme
	dl     judge.Deadlines
	stale  map[string]bool // clusters a temoin perimes au dernier jugement (SPARQL)
	seen   map[string]bool // Lease recues PENDANT un cycle (pas de reprogrammation possible)
	// lastSeen : instant du dernier evenement de Lease d'un cluster (nil/zero : inconnu)
	lastSeen func(cluster string) time.Time
}

// set memorise les echeances du jugement qui vient de finir et les arme.
func (s *deadlineState) set(d judge.Deadlines) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dl, s.active = d, true
	s.stale = map[string]bool{}
	again := false
	for _, c := range d.Stale {
		s.stale[c] = true
		if s.seen[c] { // la Lease est revenue pendant ce cycle : rejuger tout de suite
			delete(s.stale, c)
			again = true
		}
	}
	s.seen = nil
	if again {
		return time.Now()
	}
	return s.next()
}

// hold : un cycle part, il reprogrammera ; les Lease recues d'ici la ne touchent a rien.
func (s *deadlineState) hold() {
	s.mu.Lock()
	s.active, s.seen = false, map[string]bool{}
	s.mu.Unlock()
}

// resume : le cycle a echoue, on garde les echeances connues.
func (s *deadlineState) resume() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
	return s.next()
}

// renewed : une Lease est arrivee pour `cluster`. Renvoie l'echeance a armer, ok=false si rien a faire.
//   - cluster perime au dernier jugement (SPARQL) : le temoin est de retour, on rejuge tout de suite
//     (le verdict doit repasser de inconnu a normal sans attendre un autre evenement) ;
//   - sinon : l'echeance de fraicheur est repoussee, sans jugement.
func (s *deadlineState) renewed(cluster string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		if s.seen != nil {
			s.seen[cluster] = true
		}
		return time.Time{}, false
	}
	if s.stale[cluster] {
		delete(s.stale, cluster)
		return time.Now(), true
	}
	if len(s.dl.Fresh) == 0 {
		return time.Time{}, false
	}
	return s.next(), true
}

func (s *deadlineState) next() time.Time {
	d := s.dl
	fresh := make([]judge.FreshDeadline, len(d.Fresh))
	copy(fresh, d.Fresh)
	for i, f := range fresh {
		if f.Witness && s.lastSeen != nil {
			if t := s.lastSeen(f.Cluster); !t.IsZero() {
				if at := t.Add(f.Window + time.Second); at.After(fresh[i].At) {
					fresh[i].At = at
				}
			}
		}
	}
	d.Fresh = fresh
	return d.Min()
}
