/*
sylvatwin-controller : CONTROLEUR CONTINU en temps reel (exigence principale).

Remplace le mode boucle (sylvatwin-once en tmux) comme mode de fonctionnement
et comme base d'evaluation. sylvatwin-once reste un outil de debug (un seul
cycle, sans Watch).

Mode H0 (hors cluster, sur le serveur, dans tmux) : lit le management avec
--mgmt-kubeconfig, decouvre les workloads par les objets Cluster de Cluster
API (provider officiel multicluster-runtime), s'y connecte avec le Secret
kubeconfig fourni par CAPI, EN MEMOIRE, en IMPERSONATION systematique de
sylvatwin-reader. Surveille par Watch (jamais de boucle de LIST periodique) ;
un filet de securite (resync) existe mais n'est PAS le mode normal.

Usage :

	sylvatwin-controller --mgmt-kubeconfig management-cluster-kubeconfig \
	  --rules /chemin/sylvatwin-poc/rules --sparql http://127.0.0.1:7878 \
	  --track sylvatwin-flux/sylvatwin-5gc --nf-namespace sylvatwin-5gc \
	  --out ./sylvatwin-out --debounce 2s --witness-node <nom-du-noeud-workload>
*/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/orange/sylvatwin-operator/internal/judge"
	"github.com/orange/sylvatwin-operator/internal/source"
	"github.com/orange/sylvatwin-operator/internal/twin"
	"github.com/orange/sylvatwin-operator/internal/watcher"
)

func main() {
	var (
		mgmtKC    = flag.String("mgmt-kubeconfig", "", "kubeconfig du cluster de management (lecture seule)")
		mgmtName  = flag.String("mgmt-name", "management-cluster", "nom logique du cluster de management")
		rules     = flag.String("rules", "", "dossier rules/ de sylvatwin-poc")
		sparql    = flag.String("sparql", "http://127.0.0.1:7878", "endpoint SPARQL 1.1 (boucle locale uniquement)")
		track     = flag.String("track", "sylvatwin-flux/sylvatwin-5gc", "Kustomizations suivies (ns/nom, separees par des virgules)")
		nfNS      = flag.String("nf-namespace", "sylvatwin-5gc", "namespace des fonctions reseau dans les workloads")
		out       = flag.String("out", "sylvatwin-out", "dossier de sortie (dernier cycle)")
		cache     = flag.String("cache", "/opt/nidar/cache-desired", "cache des clones Git")
		validity  = flag.Float64("validity", 600, "horizon de validite par defaut (s), si aucun Watch temoin")
		debounce  = flag.Duration("debounce", 2*time.Second, "regroupement des evenements Watch avant un cycle")
		resyncNet = flag.Duration("resync-safety-net", 0, "filet de securite (0 = desactive ; N'EST PAS le mode normal)")
		kcTargets = flag.String("kubeconfig-target", "sylvatwin-flux/sylvatwin-5gc-kubeconfig=my-workload-kubeadm-capd",
			"cible des kubeconfig Flux non-CAPI, DECLAREE (ns/secret=cluster) : leur contenu n'est jamais lu")
		witnessNode = flag.String("witness-node", "", "nom du Node workload dont la Lease sert de temoin (vide = pas de temoin)")
		freshness   = flag.Duration("freshness-horizon", 30*time.Second, "H_frais (theta) : age maximal du temoin avant verdict inconnu")
		journalFile = flag.String("journal", "", "fichier JSONL des cycles et des timings (vide = stdout seulement)")
	)
	flag.Parse()
	if *mgmtKC == "" {
		fmt.Fprintln(os.Stderr, "sylvatwin-controller : --mgmt-kubeconfig requis")
		os.Exit(1)
	}

	zl := zap.New(zap.UseDevMode(true))
	log.SetLogger(zl)
	logger := log.Log.WithName("sylvatwin-controller")

	mgmt, err := clientcmd.BuildConfigFromFlags("", *mgmtKC)
	if err != nil {
		fail("kubeconfig du management illisible : " + err.Error())
	}
	mgmt.UserAgent = "sylvatwin-controller"

	var jf *os.File
	if *journalFile != "" {
		jf, err = os.OpenFile(*journalFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fail(err.Error())
		}
		defer jf.Close()
	}
	var mu sync.Mutex
	writeJournal := func(v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		defer mu.Unlock()
		fmt.Println(string(b))
		if jf != nil {
			jf.Write(b)
			jf.Write([]byte("\n"))
		}
	}

	jp := &judge.Pipeline{RulesDir: *rules, Validity: *validity}
	if *rules != "" {
		e, err := judge.NewEndpoint(*sparql)
		if err != nil {
			fail(err.Error())
		}
		jp.E = e
	}

	var tracked []twin.Tracked
	for _, s := range strings.Split(*track, ",") {
		if p := strings.SplitN(strings.TrimSpace(s), "/", 2); len(p) == 2 {
			tracked = append(tracked, twin.Tracked{Namespace: p[0], Name: p[1]})
		}
	}

	opts := watcher.Options{
		ManagementConfig: mgmt,
		ManagementName:   *mgmtName,
		NFNamespace:      *nfNS,
		Tracked:          tracked,
		CacheDir:         *cache,
		OutDir:           *out,
		JudgePipeline:    twin.Twin{Judge: jp},
		Debounce:         *debounce,
		ResyncSafetyNet:  *resyncNet,
		WitnessNode:      *witnessNode,
		FreshnessHorizon: *freshness,
		OnReport: func(trigger string, rep *twin.CycleReport, err error) {
			rec := map[string]any{"trigger": trigger, "time": time.Now().UTC().Format(time.RFC3339Nano)}
			if err != nil {
				rec["error"] = err.Error()
			} else {
				rec["clusters"] = rep.Clusters
				rec["facts"] = rep.Facts
				rec["drifts"] = rep.Drifts
				rec["verdicts"] = rep.Verdicts
				rec["durationMs"] = rep.End.Sub(rep.Start).Milliseconds()
			}
			writeJournal(rec)
			if err != nil {
				logger.Error(err, "cycle en erreur", "trigger", trigger)
			} else {
				logger.Info("cycle termine", "trigger", trigger, "resume", rep.Summary())
			}
		},
		Instrument: func(ev watcher.Timing) {
			writeJournal(map[string]any{
				"type": "timing", "cluster": ev.Cluster,
				"eventAt":             ev.EventAt.UTC().Format(time.RFC3339Nano),
				"judgeStartAt":        ev.JudgeStartAt.UTC().Format(time.RFC3339Nano),
				"judgeEndAt":          ev.JudgeEndAt.UTC().Format(time.RFC3339Nano),
				"eventToJudgeStartMs": ev.JudgeStartAt.Sub(ev.EventAt).Milliseconds(),
				"judgeDurationMs":     ev.JudgeEndAt.Sub(ev.JudgeStartAt).Milliseconds(),
				"eventToJudgeEndMs":   ev.JudgeEndAt.Sub(ev.EventAt).Milliseconds(),
			})
		},
	}
	_ = source.ParseTargets(*kcTargets) // reserve : KubeconfigTargets cote twin.Twin (desired), watcher ne l'utilise pas directement

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	w, err := watcher.New(ctx, opts)
	if err != nil {
		fail(err.Error())
	}

	if *witnessNode != "" {
		logger.Info("Watch temoin configure", "node", *witnessNode, "theta", freshness.String())
	}
	logger.Info("sylvatwin-controller demarre", "debounce", debounce.String(), "resyncSafetyNet", resyncNet.String())
	if err := w.Start(ctx); err != nil {
		fail(err.Error())
	}
	_ = ctrl.Log // garde l'import controller-runtime/log coherent avec le reste du projet
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "sylvatwin-controller :", msg)
	os.Exit(1)
}

var _ = rest.Config{}
