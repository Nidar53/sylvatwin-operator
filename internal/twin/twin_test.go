package twin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/mapper"
	"github.com/orange/sylvatwin-operator/internal/rdf"
	"github.com/orange/sylvatwin-operator/internal/source"
)

// fakeSource : source.FactSource minimal, en memoire, pour tester
// twin.Twin.Cycle sans cluster reel (mission, partie 4 : "Test du cote
// desire : une Kustomization avec kubeConfig.secretRef et une cible declaree
// (...) donne known=true").
type fakeSource struct{ snap *mapper.Snapshot }

func (f *fakeSource) Name() string                                 { return f.snap.Cluster }
func (f *fakeSource) Snapshot(ctx context.Context) *mapper.Snapshot { return f.snap }
func (f *fakeSource) Close()                                        {}

var _ source.FactSource = (*fakeSource)(nil)

// depotLocal : meme mecanisme que internal/desired/desired_test.go (petit
// depot Git jetable, UNE Kustomization simple, un Deployment replicas=3).
func depotLocal(t *testing.T) (url, sha string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git absent")
	}
	d := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v : %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "test")
	app := filepath.Join(d, "app")
	_ = os.MkdirAll(app, 0o755)
	write := func(p, s string) { _ = os.WriteFile(filepath.Join(app, p), []byte(s), 0o644) }
	write("kustomization.yaml", "resources: [dep.yaml]\n")
	write("dep.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: udm, namespace: sylvatwin-5gc}\nspec:\n  replicas: 3\n  selector: {matchLabels: {x: a}}\n  template:\n    metadata: {labels: {x: a}}\n    spec: {containers: [{name: a, image: b}]}\n")
	run("add", ".")
	run("commit", "-q", "-m", "1")
	sha = run("rev-parse", "HEAD")
	return "file://" + d, sha
}

// buildMgmtSnapshot : un Snapshot de management avec un Cluster CAPI
// (namespace=name="my-workload-kubeadm-capd"), une GitRepository (pointant sur
// le depot local) et une Kustomization avec kubeConfig.secretRef (D1 : le
// cote desire distant, inerte sans la correction D1/D2).
func buildMgmtSnapshot(gitURL, sha string) *mapper.Snapshot {
	at := time.Now().UTC()
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2", "kind": "Cluster",
		"metadata": map[string]any{"name": "my-workload-kubeadm-capd", "namespace": "my-workload-kubeadm-capd"},
		"spec":     map[string]any{"controlPlaneEndpoint": map[string]any{"host": "172.30.0.2", "port": int64(6443)}},
	}}
	gitrepo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1", "kind": "GitRepository",
		"metadata": map[string]any{"name": "sylvatwin-poc", "namespace": "sylvatwin-flux"},
		"spec":     map[string]any{"url": gitURL, "ref": map[string]any{"branch": "test"}},
		"status":   map[string]any{"artifact": map[string]any{"revision": "test@sha1:" + sha}},
	}}
	ks := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1", "kind": "Kustomization",
		"metadata": map[string]any{"name": "sylvatwin-5gc", "namespace": "sylvatwin-flux"},
		"spec": map[string]any{
			"path":       "./app",
			"sourceRef":  map[string]any{"kind": "GitRepository", "name": "sylvatwin-poc"},
			"kubeConfig": map[string]any{"secretRef": map[string]any{"name": "sylvatwin-5gc-kubeconfig"}},
		},
	}}
	return &mapper.Snapshot{Cluster: "management-cluster", Status: mapper.StatusOK,
		LastSync: at, ObservedAt: at, Objects: []*unstructured.Unstructured{cluster, gitrepo, ks}}
}

// buildWorkloadSnapshot : le Deployment OBSERVE, conforme au depot Git
// (replicas 3), sur le cluster workload nomme sous sa forme CANONIQUE.
func buildWorkloadSnapshot(cluster string) *mapper.Snapshot {
	at := time.Now().UTC()
	dep := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "udm", "namespace": "sylvatwin-5gc", "uid": "u-1"},
		"spec":     map[string]any{"replicas": int64(3)},
	}}
	return &mapper.Snapshot{Cluster: cluster, Status: mapper.StatusOK, LastSync: at, ObservedAt: at,
		Objects: []*unstructured.Unstructured{dep}}
}

// TestDesireConnu_CibleNue : mission, partie 4, test 3 (forme NUE). La cible
// declaree ("my-workload-kubeadm-capd", nom NU) doit etre resolue vers le nom
// CANONIQUE en cherchant le Cluster CAPI correspondant (D2), ce qui rend le
// desire known=true et produit des individus desires sur les MEMES IRI que les
// observes (Option C).
func TestDesireConnu_CibleNue(t *testing.T) {
	gitURL, sha := depotLocal(t)
	wlCanon := iri.CanonicalCluster("my-workload-kubeadm-capd", "my-workload-kubeadm-capd")
	mgmt := buildMgmtSnapshot(gitURL, sha)
	wl := buildWorkloadSnapshot(wlCanon)

	tw := &Twin{
		ManagementName: "management-cluster",
		Sources:        []source.FactSource{&fakeSource{mgmt}, &fakeSource{wl}},
		CacheDir:       t.TempDir(),
		OutDir:         t.TempDir(),
		Tracked:        []Tracked{{Namespace: "sylvatwin-flux", Name: "sylvatwin-5gc"}},
		KubeconfigTargets: map[string]string{
			"sylvatwin-flux/sylvatwin-5gc-kubeconfig": "my-workload-kubeadm-capd", // NOM NU
		},
	}
	rep, err := tw.Cycle(context.Background())
	if err != nil {
		t.Fatalf("Cycle: %v", err)
	}
	if len(rep.Desired) != 1 || !rep.Desired[0].Known {
		t.Fatalf("desire attendu known=true, obtenu : %+v", rep.Desired)
	}
	if rep.Desired[0].Objects == 0 {
		t.Fatal("desire known mais 0 objets rendus")
	}
	for _, e := range rep.Errors {
		t.Errorf("erreur partielle inattendue : %s", e)
	}

	// Les faits desiredReplicas et l'observe doivent etre sur la MEME IRI.
	facts, err := os.ReadFile(filepath.Join(tw.OutDir, "facts.nt"))
	if err != nil {
		t.Fatal(err)
	}
	depIRI := iri.Resource(wlCanon, "sylvatwin-5gc", "Deployment", "apps", "udm")
	if !strings.Contains(string(facts), "<"+depIRI+">") {
		t.Fatalf("IRI du Deployment attendue absente des faits : %s", depIRI)
	}
	if !strings.Contains(string(facts), rdf.C("desiredReplicas").Value()) {
		t.Fatal("desiredReplicas absent des faits malgre known=true")
	}
	if !strings.Contains(string(facts), rdf.C("observedSpecReplicas").Value()) {
		t.Fatal("observedSpecReplicas absent des faits")
	}
}

// TestDesireConnu_CibleCanonique : meme scenario, mais la cible declaree est
// DEJA sous forme canonique ("ns/nom") : doit aussi fonctionner (pas de
// verification contre le management necessaire dans ce cas, cf. ResolveTargets).
func TestDesireConnu_CibleCanonique(t *testing.T) {
	gitURL, sha := depotLocal(t)
	wlCanon := iri.CanonicalCluster("my-workload-kubeadm-capd", "my-workload-kubeadm-capd")
	mgmt := buildMgmtSnapshot(gitURL, sha)
	wl := buildWorkloadSnapshot(wlCanon)

	tw := &Twin{
		ManagementName: "management-cluster",
		Sources:        []source.FactSource{&fakeSource{mgmt}, &fakeSource{wl}},
		CacheDir:       t.TempDir(),
		OutDir:         t.TempDir(),
		Tracked:        []Tracked{{Namespace: "sylvatwin-flux", Name: "sylvatwin-5gc"}},
		KubeconfigTargets: map[string]string{
			"sylvatwin-flux/sylvatwin-5gc-kubeconfig": wlCanon, // DEJA canonique
		},
	}
	rep, err := tw.Cycle(context.Background())
	if err != nil {
		t.Fatalf("Cycle: %v", err)
	}
	if len(rep.Desired) != 1 || !rep.Desired[0].Known {
		t.Fatalf("desire attendu known=true, obtenu : %+v", rep.Desired)
	}
	for _, e := range rep.Errors {
		t.Errorf("erreur partielle inattendue : %s", e)
	}
}

// TestCibleIntrouvable : la cible declaree nomme un cluster qui n'existe pas
// dans le management -> desire inconnu + erreur EXPLICITE dans rep.Errors.
func TestCibleIntrouvable(t *testing.T) {
	gitURL, sha := depotLocal(t)
	mgmt := buildMgmtSnapshot(gitURL, sha)

	tw := &Twin{
		ManagementName: "management-cluster",
		Sources:        []source.FactSource{&fakeSource{mgmt}},
		CacheDir:       t.TempDir(),
		OutDir:         t.TempDir(),
		Tracked:        []Tracked{{Namespace: "sylvatwin-flux", Name: "sylvatwin-5gc"}},
		KubeconfigTargets: map[string]string{
			"sylvatwin-flux/sylvatwin-5gc-kubeconfig": "cluster-qui-n-existe-pas",
		},
	}
	rep, err := tw.Cycle(context.Background())
	if err != nil {
		t.Fatalf("Cycle: %v", err)
	}
	if len(rep.Desired) != 1 || rep.Desired[0].Known {
		t.Fatalf("desire attendu known=false, obtenu : %+v", rep.Desired)
	}
	found := false
	for _, e := range rep.Errors {
		if strings.Contains(e, "cluster-qui-n-existe-pas") {
			found = true
		}
	}
	if !found {
		t.Fatalf("erreur explicite attendue sur la cible introuvable, rep.Errors=%v", rep.Errors)
	}
}

// TestCibleAmbigue : deux Cluster CAPI portent le MEME nom nu, dans des
// namespaces differents -> on n'invente rien, erreur EXPLICITE, desire inconnu.
func TestCibleAmbigue(t *testing.T) {
	gitURL, sha := depotLocal(t)
	mgmt := buildMgmtSnapshot(gitURL, sha)
	// Deuxieme Cluster CAPI, MEME nom, AUTRE namespace.
	dup := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2", "kind": "Cluster",
		"metadata": map[string]any{"name": "my-workload-kubeadm-capd", "namespace": "autre-ns"},
		"spec":     map[string]any{"controlPlaneEndpoint": map[string]any{"host": "172.30.0.9", "port": int64(6443)}},
	}}
	mgmt.Objects = append(mgmt.Objects, dup)

	tw := &Twin{
		ManagementName: "management-cluster",
		Sources:        []source.FactSource{&fakeSource{mgmt}},
		CacheDir:       t.TempDir(),
		OutDir:         t.TempDir(),
		Tracked:        []Tracked{{Namespace: "sylvatwin-flux", Name: "sylvatwin-5gc"}},
		KubeconfigTargets: map[string]string{
			"sylvatwin-flux/sylvatwin-5gc-kubeconfig": "my-workload-kubeadm-capd", // nom nu AMBIGU
		},
	}
	rep, err := tw.Cycle(context.Background())
	if err != nil {
		t.Fatalf("Cycle: %v", err)
	}
	if len(rep.Desired) != 1 || rep.Desired[0].Known {
		t.Fatalf("desire attendu known=false (cible ambigue), obtenu : %+v", rep.Desired)
	}
	found := false
	for _, e := range rep.Errors {
		if strings.Contains(e, "ambigu") {
			found = true
		}
	}
	if !found {
		t.Fatalf("erreur explicite 'ambigu' attendue, rep.Errors=%v", rep.Errors)
	}
}

// A7 : causes racines et impacts sont comptes separement, par verdict.
func TestByClassAndVerdict(t *testing.T) {
	got := byClassAndVerdict([]string{
		"ConfirmedViolation StaleConfigDrift/management-cluster/x/Pod/a",
		"ConfirmedViolation StaleConfigDrift/management-cluster/x/Pod/b",
		"UnknownVerdict FunctionImpactDrift/StaleConfigDrift/c/ns/Pod/a/c/ns/NetworkFunction/nrf",
	})
	if got["StaleConfigDrift"]["ConfirmedViolation"] != 2 || got["FunctionImpactDrift"]["UnknownVerdict"] != 1 || len(got) != 2 {
		t.Fatalf("%v", got)
	}
}
