package source

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Witness : Watch TEMOIN d'une seule Lease de noeud (partie 2, P1).
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
	cluster   string
	leaseName string
	theta     time.Duration

	mu       sync.RWMutex
	lastSeen time.Time
}

// NewWitness demarre immediatement le Watch (bloquant dans une goroutine
// interne, relance automatique en cas de coupure). cfg n'est jamais
// journalise ni stocke au-dela de la construction du client.
func NewWitness(ctx context.Context, cluster, nodeName string, theta time.Duration, cfg *rest.Config) (*Witness, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	w := &Witness{cluster: cluster, leaseName: nodeName, theta: theta}
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
		// Snapshot initial (evite d'attendre le premier renouvellement apres
		// un redemarrage de l'operateur).
		if obj, err := dyn.Resource(gvrLease).Namespace(ns).Get(ctx, w.leaseName, metav1.GetOptions{}); err == nil {
			w.observe(obj)
		}
		res, err := dyn.Resource(gvrLease).Namespace(ns).Watch(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + w.leaseName,
		})
		if err != nil {
			time.Sleep(5 * time.Second)
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
	w.mu.Unlock()
}

// LastSeen : u_c(t), zero si aucun evenement recu depuis le demarrage.
func (w *Witness) LastSeen() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSeen
}

// Name : nom de la Lease temoin.
func (w *Witness) Name() string { return w.leaseName }

// Theta : horizon de fraicheur configure.
func (w *Witness) Theta() time.Duration { return w.theta }
