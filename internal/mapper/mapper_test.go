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

// wlCanon : nom CANONIQUE (D2) du cluster workload de la photo reelle, SOUS LA
// MEME FORME que le provider cluster-api de multicluster-runtime (namespace du
// Cluster CAPI "my-workload-kubeadm-capd" + son nom "my-workload-kubeadm-capd",
// cf. testdata/snapshot-20261108/management-cluster.json). Avant la correction
// D2, la photo utilisait a tort le nom NU comme Snapshot.Cluster du workload :
// cela "marchait" par COINCIDENCE sur ce banc (namespace == nom du Cluster
// CAPI), masquant le bug reellement present en production (cf. HANDOFF.md,
// 2 MachineNodeMismatchDrift observes avant ce fix). Le nom canonique est celui
// qu'un VRAI watcher (internal/watcher) donne a la *source.Live engagee
// (multicluster.ClusterName = "namespace/nom", meme chose que
// iri.CanonicalCluster) : la photo doit refleter cette realite.
var wlCanon = iri.CanonicalCluster("my-workload-kubeadm-capd", "my-workload-kubeadm-capd")

// photo : entrees du mapper telles que la source live les produirait.
func photo(t *testing.T) Input {
	at := time.Date(2026, 11, 8, 15, 2, 17, 0, time.UTC)
	wl := &Snapshot{Cluster: wlCanon, Endpoint: "172.30.0.2:6443", Status: StatusOK,
		LastSync: at, ObservedAt: at, Objects: load(t, "my-workload-kubeadm-capd"),
		Scopes: []Scope{{"Node", "*"}, {"Namespace", "*"}, {"Pod", "sylvatwin-5gc"}, {"ReplicaSet", "sylvatwin-5gc"},
			{"Deployment", "sylvatwin-5gc"}, {"Secret", "sylvatwin-5gc"}}}
	mg := &Snapshot{Cluster: "management-cluster", Endpoint: "172.30.0.3:6443", Status: StatusOK,
		LastSync: at, ObservedAt: at, Objects: load(t, "management-cluster"),
		Scopes: []Scope{{"Machine", "*"}, {"Cluster", "*"}, {"Namespace", "*"}, {"GitRepository", "*"},
			{"Kustomization", "*"}, {"HelmRelease", "*"}}}
	return Input{Snapshots: []*Snapshot{wl, mg}, Flux: FluxContext{
		KubeconfigTargets:  map[string]string{"sylvatwin-flux/sylvatwin-5gc-kubeconfig": wlCanon},
		HelmChartRevisions: map[string]string{"my-workload-kubeadm-capd/my-workload-kubeadm-capd-cluster": "0.0.0+6cc241e1c3b7"},
	}}
}

func TestChaineL2(t *testing.T) {
	g := Build(photo(t))
	w := wlCanon
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

// TestMachineHasNodeViseLeNodeObserve : D2/fix. L'IRI du Node vise par
// machineHasNode (ecrit par mapL1 depuis le cluster de management, a partir
// de Machine.spec.clusterName + Machine.namespace) doit etre EXACTEMENT
// celle de l'individu Node produit par mapL2 pour le MEME instantane workload
// (a partir de Node.metadata.name, avec Snapshot.Cluster = nom canonique).
// Avant le fix D2, ces deux IRI ne coincidaient PAS en general (seulement par
// accident sur ce banc precis ou namespace == nom du Cluster CAPI) : la regle
// MachineNodeMismatchDrift (rules/derives/08-machine-node-drift.rq) considere
// alors le Node comme "sans Machine qui le revendique" (ou vice versa) et
// produit une derive ARTEFACT.
func TestMachineHasNodeViseLeNodeObserve(t *testing.T) {
	g := Build(photo(t))
	machine := rdf.IRI(iri.Resource("management-cluster", "my-workload-kubeadm-capd", "Machine", "cluster.x-k8s.io", "my-workload-kubeadm-capd-control-plane-6w58l"))
	targets := g.Objects(machine, rdf.C("machineHasNode"))
	if len(targets) != 1 {
		t.Fatalf("machineHasNode : attendu exactement 1 cible, trouve %d (%v)", len(targets), targets)
	}
	gotNodeIRI := targets[0]
	// L'individu Node REELLEMENT produit par mapL2 pour cet instantane workload.
	nodeIRI := rdf.IRI(iri.Resource(wlCanon, "", "Node", "", "my-workload-kubeadm-capd-control-plane-6w58l"))
	if gotNodeIRI != nodeIRI {
		t.Fatalf("machineHasNode vise %v, attendu EXACTEMENT l'IRI du Node observe %v", gotNodeIRI, nodeIRI)
	}
	// Et cette IRI doit bien exister comme individu :Node avec un :uid dans le
	// graphe (sinon la garde P3 de la regle 08 resterait trompee autrement).
	if !g.Has(nodeIRI, rdf.Type, rdf.C("Node")) {
		t.Fatal("l'IRI visee par machineHasNode n'est pas un individu :Node du graphe")
	}
	if len(g.Objects(nodeIRI, rdf.C("uid"))) != 1 {
		t.Fatal("l'individu Node vise par machineHasNode doit porter un :uid")
	}
}

// TestMachineHasNode_ClusterDeManagementLuiMeme : NON-REGRESSION (trouvee en
// verifiant sur le vrai banc, 13/11/2026). Le cluster de management est
// LUI-MEME decrit par un Cluster CAPI (ex. "sylva-system/management-cluster")
// dont le nom CANONIQUE ("sylva-system/management-cluster") NE correspond
// PAS au nom logique reel du FactSource management (--mgmt-name,
// INDEPENDANT, ici "management-cluster" tout court). Un calcul aveugle
// iri.CanonicalCluster(namespace, nom) pour TOUTE Machine casserait donc
// machineHasNode pour la Machine qui decrit le management : elle viserait un
// Node sous une IRI qui n'existe nulle part (faux MachineNodeMismatchDrift,
// regression constatee lors de la verification sur le banc reel avant ce
// test). resolveDescribedClusterName doit reconnaitre ce cas via
// l'endpoint (host:port) du Cluster CAPI == celui du Snapshot courant.
func TestMachineHasNode_ClusterDeManagementLuiMeme(t *testing.T) {
	at := time.Now().UTC()
	mgmtCAPICluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2", "kind": "Cluster",
		"metadata": map[string]any{"name": "management-cluster", "namespace": "sylva-system"},
		"spec":     map[string]any{"controlPlaneEndpoint": map[string]any{"host": "172.30.0.3", "port": int64(6443)}},
	}}
	mgmtMachine := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2", "kind": "Machine",
		"metadata": map[string]any{"name": "management-cluster-control-plane-gn9sx", "namespace": "sylva-system"},
		"spec":     map[string]any{"clusterName": "management-cluster"},
		"status":   map[string]any{"nodeRef": map[string]any{"name": "management-cluster-control-plane-gn9sx"}},
	}}
	mgmtNode := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]any{"name": "management-cluster-control-plane-gn9sx", "uid": "node-uid-1"},
	}}
	mg := &Snapshot{Cluster: "management-cluster", Endpoint: "172.30.0.3:6443", Status: StatusOK,
		LastSync: at, ObservedAt: at,
		Objects: []*unstructured.Unstructured{mgmtCAPICluster, mgmtMachine, mgmtNode},
		Scopes:  []Scope{{"Machine", "*"}, {"Cluster", "*"}, {"Node", "*"}}}

	g := Build(Input{Snapshots: []*Snapshot{mg}})
	machine := rdf.IRI(iri.Resource("management-cluster", "sylva-system", "Machine", "cluster.x-k8s.io", "management-cluster-control-plane-gn9sx"))
	targets := g.Objects(machine, rdf.C("machineHasNode"))
	if len(targets) != 1 {
		t.Fatalf("machineHasNode : attendu exactement 1 cible, trouve %d (%v)", len(targets), targets)
	}
	// L'IRI attendue utilise le nom LOGIQUE REEL du management ("management-cluster"),
	// PAS le nom canonique "sylva-system/management-cluster" (qui serait un artefact).
	wantNodeIRI := rdf.IRI(iri.Resource("management-cluster", "", "Node", "", "management-cluster-control-plane-gn9sx"))
	if targets[0] != wantNodeIRI {
		t.Fatalf("machineHasNode vise %v, attendu %v (nom logique reel du management, pas le nom canonique)", targets[0], wantNodeIRI)
	}
	if !g.Has(wantNodeIRI, rdf.Type, rdf.C("Node")) {
		t.Fatal("l'IRI visee par machineHasNode n'est pas l'individu :Node reellement observe")
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
	in.Desired = []*Desired{{Known: false, TargetCluster: wlCanon, Objects: []*unstructured.Unstructured{dep}}}
	g := Build(in)
	d := rdf.IRI(iri.Resource(wlCanon, "sylvatwin-5gc", "Deployment", "apps", "udm"))
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
	in.Desired = []*Desired{{Known: true, TargetCluster: wlCanon,
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

// A6 : le caractere optionnel d'une reference a un Secret est ecrit comme un fait.
func TestSecretRefInfo_Optionnel(t *testing.T) {
	obj := map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
		"volumes": []any{
			map[string]any{"secret": map[string]any{"secretName": "opt-seul", "optional": true}},
			map[string]any{"secret": map[string]any{"secretName": "mixte", "optional": true}},
			map[string]any{"secret": map[string]any{"secretName": "requis"}},
		},
		"containers": []any{map[string]any{
			"env":     []any{map[string]any{"valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "mixte"}}}},
			"envFrom": []any{map[string]any{"secretRef": map[string]any{"name": "env-opt", "optional": true}}},
		}},
		"imagePullSecrets": []any{map[string]any{"name": "pull"}},
	}}}}
	got := secretRefInfo(obj, "spec", "template", "spec")
	want := map[string]bool{"opt-seul": true, "mixte": false, "requis": false, "env-opt": true, "pull": false}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s : optionnel=%v, attendu %v", k, got[k], v)
		}
	}
}
