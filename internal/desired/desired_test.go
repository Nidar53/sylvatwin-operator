package desired

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSHA(t *testing.T) {
	if got := SHA("test@sha1:6f6fb1151e2b87aa1927079578b2aeed56c04259"); got != "6f6fb1151e2b87aa1927079578b2aeed56c04259" {
		t.Fatalf("SHA = %q", got)
	}
	if SHA("0.0.0+6cc241e1c3b7") != "" {
		t.Fatal("une version de chart n'est pas un sha")
	}
}

// depotLocal : petit depot Git jetable avec une kustomization a generateur.
func depotLocal(t *testing.T) (url, sha1, sha2 string) {
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
	_ = os.MkdirAll(filepath.Join(app, "config"), 0o755)
	write := func(p, s string) { _ = os.WriteFile(filepath.Join(app, p), []byte(s), 0o644) }
	write("kustomization.yaml", "namespace: ns1\nresources: [dep.yaml]\ngeneratorOptions: {disableNameSuffixHash: true}\nconfigMapGenerator:\n- {name: a-config, files: [a.yaml=config/a.yaml]}\n")
	write("config/a.yaml", "plmn: 999\n")
	write("dep.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: a}\nspec:\n  replicas: 2\n  selector: {matchLabels: {x: a}}\n  template:\n    metadata: {labels: {x: a}}\n    spec: {containers: [{name: a, image: b}]}\n")
	run("add", ".")
	run("commit", "-q", "-m", "1")
	sha1 = run("rev-parse", "HEAD")
	write("dep.yaml", strings.Replace(mustRead(t, filepath.Join(app, "dep.yaml")), "replicas: 2", "replicas: 5", 1))
	run("commit", "-q", "-am", "2")
	sha2 = run("rev-parse", "HEAD")
	return "file://" + d, sha1, sha2
}

func mustRead(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildRevisionExacte(t *testing.T) {
	url, sha1, sha2 := depotLocal(t)
	cache := t.TempDir()
	// Revision ANCIENNE (sha1) alors que la branche est en sha2 : on doit
	// obtenir l'etat de sha1 (replicas 2), pas la tete de branche.
	r, err := Build(context.Background(), cache, Source{URL: url, Branch: "test", Revision: "test@sha1:" + sha1, Path: "./app", TargetNamespace: "cible"})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Known {
		t.Fatalf("desire inconnu : %s", r.Reason)
	}
	var reps int64 = -1
	var cm bool
	for _, o := range r.Objects {
		if o.GetKind() == "Deployment" {
			v, _, _ := unstructuredInt(o.Object, "spec", "replicas")
			reps = v
			if o.GetNamespace() != "cible" {
				t.Errorf("targetNamespace non applique : %q", o.GetNamespace())
			}
		}
		if o.GetKind() == "ConfigMap" && o.GetName() == "a-config" {
			cm = true
		}
	}
	if reps != 2 {
		t.Errorf("replicas = %d, attendu 2 (etat du sha demande)", reps)
	}
	if !cm {
		t.Error("ConfigMap generee absente : kustomize build non applique")
	}
	_ = sha2
}

func TestShaInexistant(t *testing.T) {
	url, _, _ := depotLocal(t)
	_, err := Build(context.Background(), t.TempDir(), Source{URL: url, Branch: "test",
		Revision: "test@sha1:0000000000000000000000000000000000000000", Path: "./app"})
	if err == nil {
		t.Fatal("un sha absent du depot doit etre une erreur (desire non construit)")
	}
}

func unstructuredInt(o map[string]any, path ...string) (int64, bool, error) {
	cur := any(o)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return 0, false, nil
		}
		cur = m[p]
	}
	switch v := cur.(type) {
	case int64:
		return v, true, nil
	case int:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	}
	return 0, false, nil
}
