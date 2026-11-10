/*
Package watcher : contrôleur continu EN TEMPS RÉEL, piloté par le Watch API.

Remplace le mode boucle (sylvatwin-once en tmux, outil de debug uniquement) par
un contrôleur multi-cluster construit sur sigs.k8s.io/multicluster-runtime v0.24.1 :

	manager multi-cluster (local = management, lu directement)
	  + provider cluster-api officiel (sigs.k8s.io/multicluster-runtime/providers/
	    cluster-api) : découvre les Cluster CAPI v1beta2, se connecte via leur
	    Secret <cluster>-kubeconfig (lu EN MÉMOIRE par le provider, jamais par ce
	    paquet), engage chaque cluster prêt (ControlPlaneInitialized), avec
	    IMPERSONATION systématique de sylvatwin-reader
	  + UN controller "trigger" qui Watches tous les GVK de la portée (mêmes
	    mapper.ManagementSpecs / WorkloadSpecs que le mode --once) sur TOUS les
	    clusters engagés, et qui, sur CHAQUE événement, programme un cycle du
	    jumeau après un debounce court (1 à 2 s)

INVARIANT : ce paquet ne contient AUCUNE règle de détection. Le reconciler ne
fait qu'observer (via twin.Twin, qui appelle mapper puis judge) et publier la
fraîcheur. La détection reste entièrement dans rules/ (sylvatwin-poc).

LIMITE ASSUMÉE (documentée dans le rapport) : ce paquet ne reconstruit PAS le
graphe de façon incrémentale à partir du cache des informers. Au déclenchement,
un cycle complet revoit tout (LIST via FactSource.Snapshot). Le gain de
l'architecture Watch porte sur le DÉCLENCHEMENT (événementiel, en quelques
secondes), pas sur le coût de la reconstruction.
*/
package watcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcluster "sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mchandler "sigs.k8s.io/multicluster-runtime/pkg/handler"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
	capiprovider "sigs.k8s.io/multicluster-runtime/providers/cluster-api"

	"github.com/orange/sylvatwin-operator/internal/source"
	"github.com/orange/sylvatwin-operator/internal/twin"
)

// Options de construction du watcher.
type Options struct {
	ManagementConfig *rest.Config
	ManagementName   string
	NFNamespace      string
	Tracked          []twin.Tracked
	CacheDir         string
	OutDir           string
	JudgePipeline    twin.Twin // gabarit : Judge, CacheDir, OutDir, Tracked recopies
	Debounce         time.Duration
	// ResyncSafetyNet : filet de securite. 0 = desactive.
	ResyncSafetyNet time.Duration
	// OnReport est appele apres chaque cycle (journal, status TwinSync...).
	OnReport func(trigger string, rep *twin.CycleReport, err error)
	// Instrument recoit les trois instants d'un cycle declenche par un
	// evenement Watch (partie 1.3).
	Instrument func(ev Timing)
}

// Timing : les trois instants demandes (injection -> Watch -> fin du jugement).
// EventAt est l'instant de reception cote operateur (pas l'injection elle-meme,
// mesuree a part par les scripts de scenario).
type Timing struct {
	Cluster      string
	EventAt      time.Time
	JudgeStartAt time.Time
	JudgeEndAt   time.Time
}

// Watcher : l'etat du controleur continu.
type Watcher struct {
	opts     Options
	mgr      mcmanager.Manager
	provider *capiprovider.Provider

	mu       sync.Mutex
	pending  bool
	timer    *time.Timer
	firstEvt time.Time
	trigSrc  string
}

// specsFor : la portee Watch pour un cluster (management ou workload). Memes
// GVK que le mode --once (mapper.ManagementSpecs / WorkloadSpecs) : une seule
// source de verite pour la portee observee.
func specsFor(local bool, nfNS string) []source.Spec {
	if local {
		specs := append([]source.Spec{}, source.ManagementSpecs()...)
		specs = append(specs,
			source.Spec{Kind: "Node", Version: "v1", Resource: "nodes"},
			source.Spec{Kind: "Pod", Version: "v1", Resource: "pods"},
			source.Spec{Kind: "Deployment", Group: "apps", Version: "v1", Resource: "deployments"},
			source.Spec{Kind: "ReplicaSet", Group: "apps", Version: "v1", Resource: "replicasets"},
			source.Spec{Kind: "ConfigMap", Version: "v1", Resource: "configmaps", NoScope: true},
			source.Spec{Kind: "Secret", Version: "v1", Resource: "secrets", MetadataOnly: true, NoScope: true},
		)
		return specs
	}
	return source.WorkloadSpecs(nfNS)
}

func gvkOf(sp source.Spec) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: sp.Group, Version: sp.Version, Kind: sp.Kind}
}

// New construit le manager multi-cluster, le provider CAPI et enregistre le
// controller trigger. N'appelle pas Start.
func New(ctx context.Context, o Options) (*Watcher, error) {
	if o.Debounce <= 0 {
		o.Debounce = 2 * time.Second
	}
	w := &Watcher{opts: o}

	mgr, err := mcmanager.New(o.ManagementConfig, nil, mcmanager.Options{})
	if err != nil {
		return nil, fmt.Errorf("manager multi-cluster : %w", err)
	}
	w.mgr = mgr

	localHost := hostPort(o.ManagementConfig.Host)
	w.provider = capiprovider.New(capiprovider.Options{
		IsReady: func(_ context.Context, ccl client.Object) bool {
			c, ok := ccl.(*capiv1beta2.Cluster)
			if !ok || c.Status.Initialization.ControlPlaneInitialized == nil || !*c.Status.Initialization.ControlPlaneInitialized {
				return false
			}
			ep := fmt.Sprintf("%s:%d", c.Spec.ControlPlaneEndpoint.Host, orDefault(c.Spec.ControlPlaneEndpoint.Port, 6443))
			return ep != localHost // le management est deja observe en local : pas de doublon
		},
		NewCluster: func(_ context.Context, _ client.Object, cfg *rest.Config, opts ...crcluster.Option) (crcluster.Cluster, error) {
			cfg = rest.CopyConfig(cfg)
			cfg.Impersonate = rest.ImpersonationConfig{UserName: source.ReaderUser()}
			cfg.UserAgent = "sylvatwin-watcher"
			return crcluster.New(cfg, opts...)
		},
	})
	if err := w.provider.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("provider cluster-api : %w", err)
	}

	bld := mcbuilder.ControllerManagedBy(mgr).Named("sylvatwin-trigger")
	handlerFor := func() mchandler.TypedEventHandlerFunc[client.Object, mcreconcile.Request] {
		return mchandler.TypedEnqueueRequestsFromMapFunc[client.Object](
			func(ctx context.Context, _ client.Object) []mcreconcile.Request { return []mcreconcile.Request{{}} })
	}
	// Union des GVK du management et des workloads. Le premier devient le
	// type "For" (ancrage obligatoire du builder) ; le reste passe par
	// Watches/WatchesMetadata. Un GVK absent d'un cluster (CRD non installee,
	// ex. Kustomization sur un workload) ne bloque pas les autres : chaque
	// informer echoue independamment (log, pas de panique du manager).
	type entry struct {
		gvk          schema.GroupVersionKind
		metadataOnly bool
	}
	seen := map[schema.GroupVersionKind]bool{}
	var all []entry
	for _, local := range []bool{true, false} {
		for _, sp := range specsFor(local, o.NFNamespace) {
			g := gvkOf(sp)
			if seen[g] {
				continue
			}
			seen[g] = true
			all = append(all, entry{gvk: g, metadataOnly: sp.MetadataOnly})
		}
	}
	for i, e := range all {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(e.gvk)
		if i == 0 {
			bld = bld.For(obj)
			continue
		}
		if e.metadataOnly {
			bld = bld.WatchesMetadata(obj, handlerFor())
		} else {
			bld = bld.Watches(obj, handlerFor())
		}
	}

	if err := bld.Complete(mcreconcile.Func(func(_ context.Context, req mcreconcile.Request) (ctrl.Result, error) {
		w.onEvent(string(req.ClusterName))
		return ctrl.Result{}, nil
	})); err != nil {
		return nil, fmt.Errorf("controller trigger : %w", err)
	}
	return w, nil
}

func orDefault(v int32, d int32) int32 {
	if v == 0 {
		return d
	}
	return v
}

// onEvent : programme (ou reprogramme) un cycle apres Debounce. Les
// evenements rapproches se regroupent en UN seul cycle (dernier evenement
// gagne, fenetre glissante), evitant une rafale de reconstructions completes.
func (w *Watcher) onEvent(clusterHint string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if !w.pending {
		w.pending = true
		w.firstEvt = now
		w.trigSrc = clusterHint
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.opts.Debounce, w.fireCycle)
}

func (w *Watcher) fireCycle() {
	w.mu.Lock()
	evtAt := w.firstEvt
	src := w.trigSrc
	w.pending = false
	w.mu.Unlock()
	label := "watch"
	if src != "" {
		label = "watch:" + src
	}
	w.runCycle(label, evtAt)
}

// runCycle : execute un cycle du jumeau (twin.Twin.Cycle) avec les sources
// courantes (management + clusters engages par le provider CAPI).
func (w *Watcher) runCycle(trigger string, evtAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	judgeStart := time.Now()
	rep, err := w.buildTwin(ctx).Cycle(ctx)
	judgeEnd := time.Now()
	if w.opts.Instrument != nil && !evtAt.IsZero() {
		w.opts.Instrument(Timing{Cluster: trigger, EventAt: evtAt, JudgeStartAt: judgeStart, JudgeEndAt: judgeEnd})
	}
	if w.opts.OnReport != nil {
		w.opts.OnReport(trigger, rep, err)
	}
}

// buildTwin : reconstruit un twin.Twin a partir des FactSource actuelles.
func (w *Watcher) buildTwin(ctx context.Context) *twin.Twin {
	t := &twin.Twin{Management: w.opts.ManagementConfig, ManagementName: w.opts.ManagementName,
		Tracked: w.opts.Tracked, CacheDir: w.opts.CacheDir, OutDir: w.opts.OutDir, Judge: w.opts.JudgePipeline.Judge}

	if local, err := w.mgr.GetCluster(ctx, mcmanager.LocalCluster); err == nil {
		if ls, e := source.NewLive(w.opts.ManagementName, local.GetConfig(), specsFor(true, w.opts.NFNamespace)); e == nil {
			t.Sources = append(t.Sources, ls)
		}
	}
	for _, name := range w.provider.ListClusters() {
		cl, err := w.mgr.GetCluster(ctx, name)
		if err != nil {
			continue
		}
		if ws, err := source.NewLive(string(name), cl.GetConfig(), specsFor(false, w.opts.NFNamespace)); err == nil {
			t.Sources = append(t.Sources, ws)
		}
	}
	return t
}

// EngagedClusters : noms des clusters actuellement engages (hors management).
// Utile pour le status et les tests.
func (w *Watcher) EngagedClusters() []string {
	var out []string
	for _, n := range w.provider.ListClusters() {
		out = append(out, string(n))
	}
	return out
}

// Start bloque jusqu'a ce que le contexte soit annule.
func (w *Watcher) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	if w.opts.ResyncSafetyNet > 0 {
		go func() {
			t := time.NewTicker(w.opts.ResyncSafetyNet)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					logger.Info("filet de securite : resync periodique (aucun evenement Watch recent)")
					w.runCycle("resync-safety-net", time.Time{})
				}
			}
		}()
	}
	return w.mgr.Start(ctx)
}

// LocalClusterName : alias pratique.
const LocalClusterName = mcmanager.LocalCluster

var _ = multicluster.ClusterName("")

func hostPort(h string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if !strings.Contains(s, ":") {
		s += ":443"
	}
	return s
}
