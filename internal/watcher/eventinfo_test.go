package watcher

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func obj(rv, replicas string, data string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": "s", "namespace": "ns", "resourceVersion": rv},
		"data":     map[string]any{"k": data},
		"spec":     map[string]any{"replicas": replicas},
	}}
	return u
}

// Le journal donne les NOMS des champs modifies, jamais leurs valeurs.
func TestChangedFields_NomsSeulement(t *testing.T) {
	a, b := obj("1", "1", "SECRETVALUE-A"), obj("2", "1", "SECRETVALUE-B")
	got := strings.Join(changedFields(a, b), ",")
	if got != "data" {
		t.Fatalf("attendu data, obtenu %q", got)
	}
	d := describe("wl", b, "update", changedFields(a, b))
	if strings.Contains(d, "SECRETVALUE") || !strings.Contains(d, "Secret ns/s update [data]") {
		t.Fatalf("description incorrecte : %q", d)
	}
	if g := changedFields(a, obj("3", "1", "SECRETVALUE-A")); len(g) != 1 || g[0] != "resourceVersion-seul" {
		t.Fatalf("resourceVersion seul : %v", g)
	}
}
