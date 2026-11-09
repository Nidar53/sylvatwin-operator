package mapper

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// Photo reelle (lecture seule, 09/11/2026), Secrets en metadonnees seulement.
const snap = "testdata/snapshot-20261108"

func load(t *testing.T, cluster string) []*unstructured.Unstructured {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(snap, cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var l struct{ Items []map[string]any }
	if err := json.Unmarshal(raw, &l); err != nil {
		t.Fatal(err)
	}
	out := make([]*unstructured.Unstructured, 0, len(l.Items))
	for _, it := range l.Items {
		out = append(out, &unstructured.Unstructured{Object: it})
	}
	return out
}

// photo : entrees du mapper telles que la source live les produirait.
func photo(t *testing.T) Input {
	at := time.Date(2026, 11, 8, 15, 2, 17, 0, time.UTC)
	wl := &Snapshot{Cluster: "my-workload-kubeadm-capd", Endpoint: "172.30.0.2:6443", Status: StatusOK,
		LastSync: at, ObservedAt: at, Objects: load(t, "my-workload-kubeadm-capd"),
		Scopes: []Scope{{"Node", "*"}, {"Namespace", "*"}, {"Pod", "sylvatwin-5gc"}, {"ReplicaSet", "sylvatwin-5gc"},
			{"Deployment", "sylvatwin-5gc"}, {"Secret", "sylvatwin-5gc"}}}
	mg := &Snapshot{Cluster: "management-cluster", Endpoint: "172.30.0.3:6443", Status: StatusOK,
		LastSync: at, ObservedAt: at, Objects: load(t, "management-cluster"),
		Scopes: []Scope{{"Machine", "*"}, {"Cluster", "*"}, {"Namespace", "*"}, {"GitRepository", "*"},
			{"Kustomization", "*"}, {"HelmRelease", "*"}}}
	return Input{Snapshots: []*Snapshot{wl, mg}, Flux: FluxContext{
		KubeconfigTargets:  map[string]string{"sylvatwin-flux/sylvatwin-5gc-kubeconfig": "my-workload-kubeadm-capd"},
		HelmChartRevisions: map[string]string{"my-workload-kubeadm-capd/my-workload-kubeadm-capd-cluster": "0.0.0+6cc241e1c3b7"},
	}}
}

func TestChaineL2(t *testing.T) {
	g := Build(photo(t))
	w := "my-workload-kubeadm-capd"
	pod := rdf.IRI(iri.Resource(w, "sylvatwin-5gc", "Pod", "", "udm-7cf6c5b78d-rw6fk"))
	rs := rdf.IRI(iri.Resource(w, "sylvatwin-5gc", "ReplicaSet", "apps", "udm-7cf6c5b78d"))
	dep := rdf.IRI(iri.Resource(w, "sylvatwin-5gc", "Deployment", "apps", "udm"))
	node := rdf.IRI(iri.Resource(w, "", "Node", "", "my-workload-kubeadm-capd-control-plane-6w58l"))
	sec := rdf.IRI(iri.Resource(w, "sylvatwin-5gc", "Secret", "", "udm-keys"))
	for _, c := range []struct {
		s, p, o rdf.Term
		what    string
	}{
		{pod, rdf.C("ownedBy"), rs, "Pod -> ReplicaSet (ownerReferences)"},
		{rs, rdf.C("ownedBy"), dep, "ReplicaSet -> Deployment (ownerReferences)"},
		{pod, rdf.C("runsOn"), node, "runsOn"},
		{dep, rdf.C("observedSpecReplicas"), rdf.Int(1), "spec 1"},
		{dep, rdf.C("actualReplicas"), rdf.Int(1), "readyReplicas 1"},
		{dep, rdf.C("referencesSecret"), sec, "referencesSecret"},
		{sec, rdf.Type, rdf.C("Secret"), "Secret observe"},
	} {
		if !g.Has(c.s, c.p, c.o) {
			t.Errorf("manque : %s", c.what)
		}
	}
	// aucun fait desire sans source desiree
	if len(g.Objects(dep, rdf.C("desiredReplicas"))) != 0 {
		t.Error("desiredReplicas ecrit sans source desiree")
	}
}

func TestSecretMetadonneesSeulement(t *testing.T) {
	// La photo ne contient pas .data pour le Secret ; on verifie que le mapper
	// ne produit aucun litteral qui viendrait d'un champ data, meme si present.
	in := photo(t)
	for _, o := range in.Snapshots[0].Objects {
		if kindOf(o) == "Secret" {
			o.Object["data"] = map[string]any{"cle": "U0VDUkVU"} // "SECRET"
		}
	}
	var b strings.Builder
	if err := Build(in).WriteNTriples(&b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "U0VDUkVU") || strings.Contains(b.String(), "SECRET") {
		t.Fatal("le contenu d'un Secret a fuite dans le graphe")
	}
}

func TestDesireInconnu(t *testing.T) {
	in := photo(t)
	dep := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "udm", "namespace": "sylvatwin-5gc"}, "spec": map[string]any{"replicas": int64(3)}}}
	in.Desired = []*Desired{{Known: false, TargetCluster: "my-workload-kubeadm-capd", Objects: []*unstructured.Unstructured{dep}}}
	g := Build(in)
	d := rdf.IRI(iri.Resource("my-workload-kubeadm-capd", "sylvatwin-5gc", "Deployment", "apps", "udm"))
	if len(g.Objects(d, rdf.C("desiredReplicas"))) != 0 {
		t.Fatal("desire 'inconnu' ne doit produire aucun fait desired*")
	}
	in.Desired[0].Known = true
	in.Desired[0].Revision = "test@sha1:abc"
	g = Build(in)
	if !g.Has(d, rdf.C("desiredReplicas"), rdf.Int(3)) {
		t.Fatal("desire connu : desiredReplicas attendu")
	}
}

// TestIsomorphismeExemple : BARRIERE 1. La sortie sur la photo reelle doit etre
// isomorphe a sylvatwin-core-v1-example.ttl (hors observedAt et hors ecarts
// documentes dans testdata/ecarts.md). Requiert python3 + rdflib et le depot
// sylvatwin-poc (variable SYLVATWIN_POC, defaut ../sylvatwin-poc). Ignore sinon.
func TestIsomorphismeExemple(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	ex := filepath.Join(poc, "sylvatwin-core-v1-example.ttl")
	if _, err := os.Stat(ex); err != nil {
		t.Skipf("exemple de reference introuvable (%s) : definir SYLVATWIN_POC", ex)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 absent")
	}
	in := photo(t)
	// Le desire de reference : Git @ revision de l'exemple (replicas 1 partout).
	in.Desired = []*Desired{{Known: true, TargetCluster: "my-workload-kubeadm-capd",
		Revision: "test@sha1:9099eec76ea8836c3f5ed8d11f8b9ec28323e7d4", Objects: desiredFromSnapshot(in.Snapshots[0])}}
	tmp := t.TempDir()
	out := filepath.Join(tmp, "operateur.nt")
	f, _ := os.Create(out)
	if err := Build(in).WriteNTriples(f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	cmd := exec.Command("python3", filepath.Join("testdata", "compare.py"), out, ex)
	b, err := cmd.CombinedOutput()
	t.Logf("\n%s", b)
	if err != nil {
		t.Fatalf("barriere 1 : graphes non isomorphes (voir ci-dessus)")
	}
}

// desiredFromSnapshot : pour la barriere 1 seulement, le desire = la spec des
// Deployments de la photo (l'etat sain est conforme a Git ; le vrai desire vient
// de internal/desired, teste a part).
func desiredFromSnapshot(s *Snapshot) []*unstructured.Unstructured {
	var out []*unstructured.Unstructured
	for _, o := range s.Objects {
		if kindOf(o) == "Deployment" {
			out = append(out, o.DeepCopy())
		}
	}
	return out
}
