/*
sylvatwin-once : UN cycle du jumeau (greffier + juge), puis arret.

Mode H0 (hors cluster, decision 1) : lance sur le serveur, sans CRD ni
installation. Lit le management avec --mgmt-kubeconfig (lecture seule),
decouvre les workloads par les objets Cluster de Cluster API et s'y connecte
avec le kubeconfig fourni par CAPI, EN MEMOIRE, en IMPERSONATION de
sylvatwin-system/sylvatwin-reader (droits en lecture seule).

Sorties (--out) : facts.nt, facts.ttl, drifts.nt, verdicts.nt, cycle.json.
Code de sortie : 0 = cycle complet ; 1 = erreur ; 2 = cycle complet mais un
cluster non observe ("unreachable"/"forbidden") ou desire inconnu.

Usage :

	sylvatwin-once --mgmt-kubeconfig management-cluster-kubeconfig \
	  --rules /chemin/sylvatwin-poc/rules --sparql http://127.0.0.1:7878 \
	  --track sylvatwin-flux/sylvatwin-5gc --nf-namespace sylvatwin-5gc --out ./sylvatwin-out
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/judge"
	"github.com/orange/sylvatwin-operator/internal/mapper"
	"github.com/orange/sylvatwin-operator/internal/source"
	"github.com/orange/sylvatwin-operator/internal/twin"
)

func main() {
	var (
		mgmtKC    = flag.String("mgmt-kubeconfig", "", "kubeconfig du cluster de management (lecture seule)")
		mgmtName  = flag.String("mgmt-name", "", "nom du Cluster CAPI qui decrit le management (defaut : decouvert par endpoint)")
		rules     = flag.String("rules", "", "dossier rules/ de sylvatwin-poc (vide : greffier seul)")
		sparql    = flag.String("sparql", "http://127.0.0.1:7878", "endpoint SPARQL 1.1 (boucle locale uniquement)")
		track     = flag.String("track", "sylvatwin-flux/sylvatwin-5gc", "Kustomizations suivies (ns/nom, separees par des virgules)")
		nfNS      = flag.String("nf-namespace", "sylvatwin-5gc", "namespace des fonctions reseau dans les workloads")
		out       = flag.String("out", "sylvatwin-out", "dossier de sortie")
		cache     = flag.String("cache", "/opt/nidar/cache-desired", "cache des clones Git")
		validity  = flag.Float64("validity", 600, "horizon de validite de l'information (s), pour le verdict")
		timeout   = flag.Duration("timeout", 5*time.Minute, "duree maximale du cycle")
		kcTargets = flag.String("kubeconfig-target", "sylvatwin-flux/sylvatwin-5gc-kubeconfig=my-workload-kubeadm-capd", "cible des kubeconfig Flux non-CAPI, DECLAREE (ns/secret=cluster, separes par des virgules) : leur contenu n est jamais lu")
	)
	flag.Parse()
	if *mgmtKC == "" {
		fail("--mgmt-kubeconfig requis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	mgmt, err := clientcmd.BuildConfigFromFlags("", *mgmtKC)
	if err != nil {
		fail("kubeconfig du management illisible")
	}
	mgmt.UserAgent = "sylvatwin-once"
	found, err := source.DiscoverCAPI(ctx, mgmt)
	if err != nil {
		fail("decouverte CAPI : " + err.Error())
	}
	mgmtEP := hostPort(mgmt.Host)
	srcs := []source.FactSource{}
	name := *mgmtName
	for _, d := range found {
		if d.Endpoint == mgmtEP && name == "" {
			name = d.Name // le Cluster CAPI qui decrit le management lui-meme
		}
	}
	if name == "" {
		name = "management"
	}
	ms, err := source.NewLive(name, mgmt, source.ManagementSpecs())
	if err != nil {
		fail(err.Error())
	}
	srcs = append(srcs, ms)
	for _, d := range found {
		// dedoublonnage : le management est deja observe en local
		if d.Endpoint == mgmtEP {
			continue
		}
		if !d.Ready {
			fmt.Fprintf(os.Stderr, "cluster %s/%s : plan de controle non initialise, ignore\n", d.Namespace, d.Name)
			continue
		}
		cfg, err := source.WorkloadConfig(ctx, mgmt, d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cluster %s : %v\n", d.Name, err)
			continue
		}
		cfg.UserAgent = "sylvatwin-once"
		// D2 : nom CANONIQUE (namespace/nom), meme forme que le provider
		// cluster-api de multicluster-runtime (mode continu) : une seule regle
		// de nommage des deux cotes (internal/iri.CanonicalCluster).
		wlName := iri.CanonicalCluster(d.Namespace, d.Name)
		ws, err := source.NewLive(wlName, cfg, source.WorkloadSpecs(*nfNS))
		if err != nil {
			fail(err.Error())
		}
		srcs = append(srcs, ws)
	}

	t := &twin.Twin{Management: mgmt, ManagementName: name, Sources: srcs, CacheDir: *cache, OutDir: *out,
		Discovered: found, KubeconfigTargets: source.ParseTargets(*kcTargets)}
	for _, s := range strings.Split(*track, ",") {
		if p := strings.SplitN(strings.TrimSpace(s), "/", 2); len(p) == 2 {
			t.Tracked = append(t.Tracked, twin.Tracked{Namespace: p[0], Name: p[1]})
		}
	}
	if *rules != "" {
		e, err := judge.NewEndpoint(*sparql)
		if err != nil {
			fail(err.Error())
		}
		t.Judge = &judge.Pipeline{E: e, RulesDir: *rules, Validity: *validity}
	}
	rep, err := t.Cycle(ctx)
	if err != nil {
		fail(err.Error())
	}
	fmt.Println(rep.Summary())
	for _, l := range rep.DriftList {
		fmt.Println("  derive  :", l)
	}
	for _, l := range rep.Verdict {
		fmt.Println("  verdict :", l)
	}
	for _, e := range rep.Errors {
		fmt.Fprintln(os.Stderr, "  attention :", e)
	}
	code := 0
	for _, st := range rep.Clusters {
		if st != mapper.StatusOK {
			code = 2
		}
	}
	for _, d := range rep.Desired {
		if !d.Known {
			code = 2
		}
	}
	os.Exit(code)
}

func hostPort(h string) string {
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	if i := strings.Index(h, "/"); i >= 0 {
		h = h[:i]
	}
	if !strings.Contains(h, ":") {
		h += ":443"
	}
	return h
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "sylvatwin-once :", msg)
	os.Exit(1)
}
