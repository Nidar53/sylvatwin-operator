package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	// D2 : la convention CAPI <cluster>-kubeconfig doit deduire le nom CANONIQUE
	// "namespace/nom" (meme forme que le provider multicluster-runtime), PAS le
	// seul nom nu du Cluster CAPI.
	if fc.KubeconfigTargets["wl-ns/wl-kubeconfig"] != "wl-ns/wl" {
		t.Error("convention CAPI <cluster>-kubeconfig non appliquee au nom CANONIQUE")
	}
	if fc.KubeconfigTargets["f/k"] != "wl2" || len(fc.KubeconfigTargets) != 2 {
		t.Errorf("correspondances declarees : %v", fc.KubeconfigTargets)
	}
}

// Un serveur qui accepte la connexion et ne repond jamais (connexion muette) :
// le Snapshot rend la main apres readTimeout, statut "injoignable", et le dernier
// instantane reussi est conserve.
func TestSnapshot_ServeurMuet_DelaiMaximal(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	cfg := &rest.Config{Host: srv.URL}
	l, err := NewLive("muet", cfg, []Spec{{Kind: "Pod", Version: "v1", Resource: "pods", Namespace: "*"}})
	if err != nil {
		t.Fatal(err)
	}
	l.SetReadTimeout(300 * time.Millisecond)
	start := time.Now()
	s := l.Snapshot(context.Background())
	took := time.Since(start)
	if s.Status == mapper.StatusOK {
		t.Fatalf("statut ok sur un serveur muet")
	}
	if took > 3*time.Second {
		t.Fatalf("lecture non bornee : %v", took)
	}
}
