package source

import (
	"context"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Witness : Watch TEMOIN des Lease de noeud d'UN cluster workload (partie 2, P1 ;
// A5 : un temoin par workload decouvert, sans nom de noeud a configurer).
//
// Les Lease du namespace kube-node-lease sont toutes surveillees : n'importe quel
// evenement recu prouve que le canal Watch vers ce cluster est vivant. Le cout
// est negligeable (un renouvellement par noeud toutes les ~10 s).
//
// Exigence du contrôleur (partie 1) : ne jamais surveiller les Lease par
// defaut. CE Watch est l'exception assumee et documentee : il NE sert PAS a
// detecter une derive, seulement a mesurer la fraicheur du canal
// d'observation vers un cluster (u_c(t) = dernier renouvellement RECU).
//
// Independant du cycle LIST (mapper.Snapshot) : un cluster peut etre "ok" au
// sens du dernier LIST reussi tout en ayant un canal Watch degrade ; c'est
// justement ce que le temoin permet de detecter.
var gvrLease = schema.GroupVersionResource{Group: "coordination.k8s.io", Version: "v1", Resource: "leases"}

type Witness struct {
	cluster string
	theta   time.Duration

	mu       sync.RWMutex
	lastSeen time.Time
	onRenew  func() // appele a CHAQUE evenement de Lease (hors verrou) ; sans jugement ni cycle
	cancel   context.CancelFunc
}

// OnRenew : fonction appelee a chaque evenement de Lease recu. Sert uniquement
// a repousser la minuterie de fraicheur (aucun cycle, aucun jugement).
func (w *Witness) OnRenew(f func()) {
	w.mu.Lock()
	w.onRenew = f
	w.mu.Unlock()
}

// Stop arrete le Watch (cluster disparu). Sans effet s'il est deja arrete.
func (w *Witness) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

// NewWitness demarre immediatement le Watch (bloquant dans une goroutine
// interne, relance automatique en cas de coupure). cfg n'est jamais
// journalise ni stocke au-dela de la construction du client.
func NewWitness(ctx context.Context, cluster string, theta time.Duration, cfg *rest.Config) (*Witness, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &Witness{cluster: cluster, theta: theta, cancel: cancel}
	go w.loop(ctx, dyn)
	return w, nil
}

func (w *Witness) loop(ctx context.Context, dyn dynamic.Interface) {
	ns := "kube-node-lease"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Sans resourceVersion, le Watch rejoue d'abord l'etat courant (ADDED) :
		// pas de Get a part, u_c(t) est connu des le demarrage.
		res, err := dyn.Resource(gvrLease).Namespace(ns).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			ctrllog.Log.WithName("temoin").Error(err, "Watch des Lease impossible (nouvel essai dans 5 s)", "cluster", w.cluster)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		w.drain(ctx, res)
	}
}

func (w *Witness) drain(ctx context.Context, res watch.Interface) {
	defer res.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-res.ResultChan():
			if !ok {
				return // canal ferme : le loop() exterieur relance un Watch
			}
			if u, ok := ev.Object.(*unstructured.Unstructured); ok {
				w.observe(u)
			}
		}
	}
}

func (w *Witness) observe(u *unstructured.Unstructured) {
	renew, _, _ := unstructured.NestedString(u.Object, "spec", "renewTime")
	t := time.Now().UTC() // u_c(t) = instant de RECEPTION (preuve de vie du canal),
	_ = renew             // pas l'horodatage serveur (qui ne prouve rien sur le canal local)
	w.mu.Lock()
	w.lastSeen = t
	f := w.onRenew
	w.mu.Unlock()
	if f != nil {
		f()
	}
}

// LastSeen : u_c(t), zero si aucun evenement recu depuis le demarrage.
func (w *Witness) LastSeen() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSeen
}

// Name : ce qui est surveille (ecrit comme fait :witnessLeaseName).
func (w *Witness) Name() string { return "kube-node-lease" }

// Theta : horizon de fraicheur configure.
func (w *Witness) Theta() time.Duration { return w.theta }
