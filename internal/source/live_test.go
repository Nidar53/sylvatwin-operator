package source

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/orange/sylvatwin-operator/internal/mapper"
)

func TestClassify(t *testing.T) {
	gr := schema.GroupResource{Resource: "pods"}
	if classify(apierrors.NewForbidden(gr, "x", errors.New("non"))) != mapper.StatusForbidden {
		t.Error("403 -> forbidden attendu")
	}
	if classify(apierrors.NewUnauthorized("non")) != mapper.StatusForbidden {
		t.Error("401 -> forbidden attendu")
	}
	if classify(errors.New("dial tcp 172.30.0.2:6443: connect: connection refused")) != mapper.StatusUnreachable {
		t.Error("erreur reseau -> unreachable attendu")
	}
}

// Cluster injoignable : le DERNIER instantane reussi est conserve (faits et
// portee), seul le statut change (P3). Premier echec sans instantane : vide.
func TestSnapshotInjoignableConserveLeDernier(t *testing.T) {
	l, err := NewLive("wl", &rest.Config{Host: "https://127.0.0.1:1"}, []Spec{{Kind: "Node", Version: "v1", Resource: "nodes"}})
	if err != nil {
		t.Fatal(err)
	}
	s := l.Snapshot(context.Background())
	if s.Status != mapper.StatusUnreachable || len(s.Objects) != 0 || !s.LastSync.IsZero() {
		t.Fatalf("premier echec : %+v", s)
	}
	prev := &mapper.Snapshot{Cluster: "wl", Status: mapper.StatusOK, Scopes: []mapper.Scope{{Kind: "Node", Namespace: "*"}}}
	l.last = prev
	s = l.Snapshot(context.Background())
	if s.Status != mapper.StatusUnreachable || len(s.Scopes) != 1 || l.last.Status != mapper.StatusOK {
		t.Fatalf("echec apres succes : statut=%s scopes=%d (le dernier succes doit rester intact)", s.Status, len(s.Scopes))
	}
}

func TestReaderUser(t *testing.T) {
	if ReaderUser() != "system:serviceaccount:sylvatwin-system:sylvatwin-reader" {
		t.Fatal(ReaderUser())
	}
}

func TestHostPort(t *testing.T) {
	for in, want := range map[string]string{"https://172.30.0.2:6443": "172.30.0.2:6443", "https://k.example": "k.example:443"} {
		if got := hostPort(in); got != want {
			t.Errorf("%s -> %s, attendu %s", in, got, want)
		}
	}
}

func TestFluxContextSansLireDeSecret(t *testing.T) {
	fc := FluxContext(nil, []Discovered{{Name: "wl", Namespace: "wl-ns"}}, ParseTargets("f/k=wl2, mauvais ,a/b="))
	if fc.KubeconfigTargets["wl-ns/wl-kubeconfig"] != "wl" {
		t.Error("convention CAPI <cluster>-kubeconfig non appliquee")
	}
	if fc.KubeconfigTargets["f/k"] != "wl2" || len(fc.KubeconfigTargets) != 2 {
		t.Errorf("correspondances declarees : %v", fc.KubeconfigTargets)
	}
}
