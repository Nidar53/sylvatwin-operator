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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcluster "sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/config"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

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
	// KubeconfigTargets : correspondances DECLAREES (--kubeconfig-target,
	// "ns/secret=cluster") cote desire, transmises telles quelles au Twin (D1).
	// "cluster" peut etre un nom NU (D2 : resolu vers le nom canonique par
	// twin.Twin.Cycle, qui a acces a l'instantane du management) ou deja la
	// forme canonique "namespace/nom".
	KubeconfigTargets map[string]string
	JudgePipeline     twin.Twin // gabarit : Judge, CacheDir, OutDir, Tracked recopies
	// Debounce : fenetre de fusion glissante des evenements Watch.
	Debounce time.Duration
	// MaxDelay : delai MAXIMAL entre le premier evenement en attente et le
	// depart du cycle, meme si les evenements continuent d'arriver (A2).
	// Defaut : 5 x Debounce.
	MaxDelay time.Duration
	// ResyncSafetyNet : filet de securite. 0 = desactive.
	ResyncSafetyNet time.Duration
	// WitnessNode : nom du Node DANS LE WORKLOAD dont la Lease sert de temoin
	// de fraicheur (partie 2, P1). Vide = pas de temoin. Attache automatiquement
	// a la premiere source workload engagee (H0 : un seul workload en pratique).
	WitnessNode string
	// FreshnessHorizon : H_frais (theta), publie comme fait sur le cluster temoin.
	FreshnessHorizon time.Duration
	// OnReport est appele apres chaque cycle (journal, status TwinSync...).
	OnReport func(trigger string, rep *twin.CycleReport, err error)
	// Instrument recoit les trois instants d'un cycle declenche par un
	// evenement Watch (partie 1.3).
	Instrument func(ev Timing)
	// MetricsBindAddress : adresse d'ecoute des metriques Prometheus du
	// manager. "0" ou ":0" = port libre choisi par l'OS (utile pour lancer
	// plusieurs instances sur la meme machine, ex. tests manuels paralleles
	// a un Watcher deja en cours). "" = defaut controller-runtime (:8080).
	MetricsBindAddress string
	// HostOverride (OUTIL DE TEST/DIAGNOSTIC UNIQUEMENT, partie 2) : si non
	// nil, remplace le "host:port" d'un cluster workload DECOUVERT (jamais le
	// management) par la valeur renvoyee, AVANT la connexion. Sert a simuler
	// une coupure reseau EN LECTURE SEULE, sans toucher au cluster : on fait
	// pointer le Watcher vers un proxy TCP local (127.0.0.1) que l'on coupe a
	// volonte, au lieu du vrai control-plane. Si HostOverride renvoie "", pas
	// de changement. Ne jamais utiliser en dehors d'un test : le Secret
	// kubeconfig original reste la seule source d'authentification (CA, certs
	// client), seul le host:port de connexion change.
	HostOverride func(endpoint string) string
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
	// longCtx : contexte de VIE DU WATCHER (celui passe a New/Start), PAS le
	// contexte d'un cycle (2 minutes, annule a la fin). Le Watch temoin doit
	// survivre aux cycles : demarre avec le ctx d'un cycle, sa goroutine est
	// tuee des la fin de CE cycle et lastWitnessRenewTime se fige
	// silencieusement (bug reproduit le 10/11 : verdicts passes a tort en
	// UnknownVerdict apres la premiere Cycle, cf. HANDOFF).
	longCtx context.Context

	// sched : l'UNIQUE executeur de cycles (A1 : un cycle a la fois ; A2 :
	// fusion des rafales bornee ; A3 : minuterie d'echeance). Voir scheduler.go.
	sched *scheduler

	witMu   sync.Mutex
	witness *source.Witness
	witFor  string // nom du cluster workload auquel le temoin est attache

	// srcMu/sources : UNE *source.Live par cluster, reutilisee a CHAQUE cycle
	// (pas recreee). Necessaire pour la garantie P3 (source.go : "si le
	// cluster est injoignable, renvoie le DERNIER instantane reussi") : une
	// *Live toute neuve a chaque cycle a l.last == nil, donc un cluster qui
	// tombe juste apres le premier cycle reussi ne "se souviendrait" jamais
	// du dernier succes (bug trouve en ecrivant le test envtest "cluster
	// injoignable", partie 1.4, cf. HANDOFF).
	srcMu   sync.Mutex
	sources map[string]*source.Live
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
	if o.MaxDelay <= 0 {
		o.MaxDelay = 5 * o.Debounce
	}
	w := &Watcher{opts: o, longCtx: ctx, sources: map[string]*source.Live{}, sched: newScheduler(o.Debounce, o.MaxDelay)}

	// Scheme : le manager multi-cluster en a besoin pour TOUS les types
	// utilises, y compris par le provider CAPI (capiv1beta2.Cluster), sans
	// quoi "no kind is registered for the type v1beta2.Cluster" au demarrage.
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capiv1beta2.AddToScheme(scheme))

	// Le provider CAPI doit exister AVANT le manager : mcmanager.New prend le
	// provider en 2e argument (sans lui, GetCluster(name) echoue avec "no
	// multicluster provider set", meme si le provider est branche plus tard
	// via SetupWithManager — l'erreur rencontree en essai reel du 10/11).
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
			if o.HostOverride != nil {
				if h := o.HostOverride(cfg.Host); h != "" {
					cfg.Host = h
					// Le proxy relaie vers le VRAI control-plane (meme CA, meme
					// certificat serveur, qui couvre localhost/127.0.0.1) : pas
					// besoin de desactiver la verification TLS.
				}
			}
			// sylvatwin-reader n'a les droits "secrets" que dans le namespace
			// des NF (Role sylvatwin-reader-secrets-meta), PAS cluster-wide :
			// sans restreindre le cache ici, l'informer Secret tente un LIST
			// cluster-scope et echoue en boucle ("forbidden ... at the cluster
			// scope") — erreur reproduite et corrigee le 10/11 (cf. HANDOFF).
			opts = append(opts, func(co *crcluster.Options) {
				if co.Cache.ByObject == nil {
					co.Cache.ByObject = map[client.Object]cache.ByObject{}
				}
				co.Cache.ByObject[&corev1.Secret{}] = cache.ByObject{
					Namespaces: map[string]cache.Config{o.NFNamespace: {}},
				}
			})
			return crcluster.New(cfg, opts...)
		},
	})

	// SkipNameValidation : true uniquement pour permettre aux tests envtest de
	// cette suite de creer plusieurs Watcher dans le MEME process Go (chaque
	// TestXxx cree son propre manager "cluster"/"sylvatwin-trigger", et
	// controller-runtime refuse par defaut deux controllers de meme nom dans
	// le registre de metriques global du process). Sans consequence en
	// production : H0 n'a qu'un seul Watcher par process.
	skipNameValidation := true
	mgr, err := mcmanager.New(o.ManagementConfig, w.provider, mcmanager.Options{
		Scheme:     scheme,
		Controller: config.Controller{SkipNameValidation: &skipNameValidation},
		Metrics:    metricsserver.Options{BindAddress: o.MetricsBindAddress},
	})
	if err != nil {
		return nil, fmt.Errorf("manager multi-cluster : %w", err)
	}
	w.mgr = mgr

	if err := w.provider.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("provider cluster-api : %w", err)
	}

	// WithOptions(SkipNameValidation) : meme remarque que pour le manager
	// ci-dessus (voir commentaire sur Controller.SkipNameValidation) — le
	// reglage du Manager n'est PAS herite automatiquement par ce controller
	// (controller.NewTypedUnmanaged n'a pas acces au manager), donc il faut le
	// repeter ici explicitement, sinon un 2e Watcher dans le meme process
	// (tests envtest) echoue a la creation avec "controller ... already exists".
	bld := mcbuilder.ControllerManagedBy(mgr).Named("sylvatwin-trigger").
		WithOptions(crcontroller.TypedOptions[mcreconcile.Request]{SkipNameValidation: &skipNameValidation})
	handlerFor := func() mchandler.TypedEventHandlerFunc[client.Object, mcreconcile.Request] {
		return mchandler.TypedEnqueueRequestsFromMapFunc[client.Object](
			func(ctx context.Context, _ client.Object) []mcreconcile.Request { return []mcreconcile.Request{{}} })
	}
	// Union des GVK du management et des workloads, avec la portee
	// d'engagement DECLAREE explicitement par GVK (onManagement / onWorkload) :
	// sans cela, multicluster-runtime applique un defaut surprenant des que
	// SON PROVIDER est configure (forked_controller.go) : engageWithLocalCluster
	// vaut false et engageWithProviderClusters vaut true PAR DEFAUT, meme pour
	// un GVK qui n'existe QUE sur le management (Cluster CAPI, Kustomization,
	// HelmRelease, GitRepository, Machine). Sans cette correction, le manager
	// tente d'engager ces GVK sur le cluster workload et echoue en boucle
	// ("no matches for kind ... in version ...") — bug reproduit et corrige le
	// 10/11, cf. essai reel en tmux (voir HANDOFF).
	// Le premier GVK devient le type "For" (ancrage obligatoire du builder) ;
	// le reste passe par Watches/WatchesMetadata.
	type entry struct {
		gvk          schema.GroupVersionKind
		metadataOnly bool
		onManagement bool
		onWorkload   bool
	}
	merged := map[schema.GroupVersionKind]*entry{}
	var order []schema.GroupVersionKind
	for _, local := range []bool{true, false} {
		for _, sp := range specsFor(local, o.NFNamespace) {
			g := gvkOf(sp)
			e, ok := merged[g]
			if !ok {
				e = &entry{gvk: g, metadataOnly: sp.MetadataOnly}
				merged[g] = e
				order = append(order, g)
			}
			if local {
				e.onManagement = true
			} else {
				e.onWorkload = true
			}
		}
	}
	for i, g := range order {
		e := merged[g]
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(e.gvk)
		engage := []mcbuilder.WatchesOption{
			mcbuilder.WithEngageWithLocalCluster(e.onManagement),
			mcbuilder.WithEngageWithProviderClusters(e.onWorkload),
		}
		if i == 0 {
			forOpts := make([]mcbuilder.ForOption, len(engage))
			for j, o := range engage {
				forOpts[j] = o.(mcbuilder.ForOption)
			}
			bld = bld.For(obj, forOpts...)
			continue
		}
		if e.metadataOnly {
			bld = bld.WatchesMetadata(obj, handlerFor(), engage...)
		} else {
			bld = bld.Watches(obj, handlerFor(), engage...)
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

// onEvent : un evenement Watch. Il est confie a l'ordonnanceur, qui fusionne
// les rafales (fenetre glissante bornee par MaxDelay) et garantit qu'un seul
// cycle tourne a la fois.
func (w *Watcher) onEvent(clusterHint string) { w.sched.Event(clusterHint) }

// runTrigger : execute UN cycle du jumeau (twin.Twin.Cycle) avec les sources
// courantes (management + clusters engages par le provider CAPI). Appele
// uniquement par l'executeur unique de l'ordonnanceur : jamais en parallele.
func (w *Watcher) runTrigger(tr Trigger) {
	label := "watch"
	if tr.Cluster != "" {
		label = "watch:" + tr.Cluster
	}
	if tr.Cause == CauseDeadline {
		label = CauseDeadline
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	judgeStart := time.Now()
	rep, err := w.buildTwin(ctx).Cycle(ctx)
	judgeEnd := time.Now()
	if w.opts.Instrument != nil && !tr.FirstEventAt.IsZero() {
		w.opts.Instrument(Timing{Cluster: label, EventAt: tr.FirstEventAt, JudgeStartAt: judgeStart, JudgeEndAt: judgeEnd})
	}
	if w.opts.OnReport != nil {
		w.opts.OnReport(label, rep, err)
	}
}

// liveSource : renvoie la *source.Live DEJA CONNUE pour ce nom de cluster, ou
// en cree une et la memorise. Reutilisee a chaque cycle (voir Watcher.sources)
// pour que la garantie P3 (dernier instantane reussi conserve) fonctionne
// reellement d'un cycle a l'autre.
func (w *Watcher) liveSource(name string, cfg *rest.Config, specs []source.Spec) (*source.Live, error) {
	w.srcMu.Lock()
	defer w.srcMu.Unlock()
	if ls, ok := w.sources[name]; ok {
		return ls, nil
	}
	ls, err := source.NewLive(name, cfg, specs)
	if err != nil {
		return nil, err
	}
	w.sources[name] = ls
	return ls, nil
}

// buildTwin : reconstruit un twin.Twin a partir des FactSource actuelles.
//
// D1 : KubeconfigTargets (--kubeconfig-target) est transmis au Twin (jadis
// jete par main.go). Discovered n'a PAS besoin d'etre rempli ici : twin.Twin.
// Cycle le deduit lui-meme du Snapshot du management (source.DiscoveredFromSnapshot,
// fonction pure) quand il est nil, ce qui est toujours le cas en mode continu
// (le watcher n'a pas de LIST CAPI a part, il observe deja tout par Watch).
func (w *Watcher) buildTwin(ctx context.Context) *twin.Twin {
	t := &twin.Twin{Management: w.opts.ManagementConfig, ManagementName: w.opts.ManagementName,
		Tracked: w.opts.Tracked, CacheDir: w.opts.CacheDir, OutDir: w.opts.OutDir, Judge: w.opts.JudgePipeline.Judge,
		KubeconfigTargets: w.opts.KubeconfigTargets}

	if local, err := w.mgr.GetCluster(ctx, mcmanager.LocalCluster); err == nil {
		if ls, e := w.liveSource(w.opts.ManagementName, local.GetConfig(), specsFor(true, w.opts.NFNamespace)); e == nil {
			t.Sources = append(t.Sources, ls)
		}
	}
	for _, name := range w.provider.ListClusters() {
		cl, err := w.mgr.GetCluster(ctx, name)
		if err != nil {
			continue
		}
		ws, err := w.liveSource(string(name), cl.GetConfig(), specsFor(false, w.opts.NFNamespace))
		if err != nil {
			continue
		}
		// w.longCtx (PAS ctx, qui expire a la fin de CE cycle) : le Watch
		// temoin doit survivre a la duree de vie du Watcher, pas d'un cycle.
		if wit := w.witnessFor(w.longCtx, string(name), cl.GetConfig()); wit != nil {
			ws.SetWitness(wit)
		}
		t.Sources = append(t.Sources, ws)
	}
	return t
}

// witnessFor : demarre (une seule fois) le Watch temoin sur le premier cluster
// workload engage, s'il est configure. Reutilise ensuite la meme instance.
func (w *Watcher) witnessFor(ctx context.Context, cluster string, cfg *rest.Config) *source.Witness {
	if w.opts.WitnessNode == "" {
		return nil
	}
	w.witMu.Lock()
	defer w.witMu.Unlock()
	if w.witness != nil {
		if w.witFor == cluster {
			return w.witness
		}
		return nil // temoin deja attache a un AUTRE cluster (H0 : un seul workload attendu)
	}
	theta := w.opts.FreshnessHorizon
	if theta <= 0 {
		theta = 30 * time.Second
	}
	wit, err := source.NewWitness(ctx, cluster, w.opts.WitnessNode, theta, cfg)
	if err != nil {
		return nil
	}
	w.witness, w.witFor = wit, cluster
	return wit
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
	go w.sched.Run(ctx, w.runTrigger)
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
					w.sched.Event("resync-safety-net") // passe par l'executeur unique
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
