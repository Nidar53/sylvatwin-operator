package source

// GARDE DE SECURITE (regle 4) : le code de production ne doit lire le CONTENU
// d'aucun Secret, sauf le Secret "<cluster>-kubeconfig" d'un Cluster CAPI, lu
// dans WorkloadConfig (capi.go) pour se connecter, en memoire.
//
// Deux verifications :
//  1. statique : tout appel Secrets(...).Get/List/Watch du code de production
//     (hors _test.go) doit se trouver dans capi.go, sur le nom d+"-kubeconfig" ;
//  2. dynamique : WorkloadConfig contre un faux serveur API : seule la requete
//     GET .../secrets/<cluster>-kubeconfig est emise ; les Secrets observes par
//     les sources passent par l'API metadata (PartialObjectMetadata).
//
// Origine : erreur du 09/11/2026 (lecture en memoire du Secret
// sylvatwin-flux/sylvatwin-5gc-kubeconfig), corrigee dans e131c2a.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"k8s.io/client-go/rest"
)

func TestAucuneLectureDeSecretHorsKubeconfigCAPI(t *testing.T) {
	root := filepath.Join("..", "..")
	var bad []string
	allowed := 0
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "bin", "vendor", "testdata", ".git", "test":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, p, nil, 0)
		if err != nil {
			return err
		}
		src, _ := os.ReadFile(p)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Get" && sel.Sel.Name != "List" && sel.Sel.Name != "Watch") {
				return true
			}
			inner, ok := sel.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			isel, ok := inner.Fun.(*ast.SelectorExpr)
			if !ok || isel.Sel.Name != "Secrets" {
				return true
			}
			pos := fs.Position(call.Pos())
			text := string(src[fs.Position(call.Pos()).Offset:fs.Position(call.End()).Offset])
			if filepath.Base(p) == "capi.go" && sel.Sel.Name == "Get" && strings.Contains(text, `"-kubeconfig"`) {
				allowed++
				return true
			}
			bad = append(bad, pos.String()+" : "+text)
			return true
		})
		// Acces generique a la ressource "secrets" par le client dynamique
		// (contournement possible du controle ci-dessus).
		if strings.Contains(string(src), `Resource: "secrets"`) && filepath.Base(p) != "source.go" {
			bad = append(bad, p+" : ressource \"secrets\" lue par le client dynamique")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("lecture de Secret non autorisee :\n%s", strings.Join(bad, "\n"))
	}
	if allowed != 1 {
		t.Fatalf("attendu exactement 1 lecture autorisee (<cluster>-kubeconfig dans capi.go), trouve %d", allowed)
	}
	// Dans source.go, la seule Spec "secrets" doit etre en metadonnees seulement.
	for _, sp := range WorkloadSpecs("ns") {
		if sp.Resource == "secrets" && !sp.MetadataOnly {
			t.Fatal("Spec secrets sans MetadataOnly")
		}
	}
}

// Faux serveur API : enregistre les chemins demandes.
func TestWorkloadConfigNeLitQueLeKubeconfigCAPI(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	kc := "apiVersion: v1\nkind: Config\nclusters: [{name: c, cluster: {server: 'https://172.30.0.2:6443'}}]\n" +
		"users: [{name: u, user: {token: abc}}]\ncontexts: [{name: x, context: {cluster: c, user: u}}]\ncurrent-context: x\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/namespaces/wl-ns/secrets/wl-kubeconfig" {
			_ = json.NewEncoder(w).Encode(map[string]any{"apiVersion": "v1", "kind": "Secret",
				"metadata": map[string]any{"name": "wl-kubeconfig", "namespace": "wl-ns"},
				"data":     map[string]any{"value": base64.StdEncoding.EncodeToString([]byte(kc))}})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	cfg, err := WorkloadConfig(context.Background(), &rest.Config{Host: srv.URL}, Discovered{Name: "wl", Namespace: "wl-ns"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Impersonate.UserName != ReaderUser() {
		t.Fatalf("impersonation absente : %q", cfg.Impersonate.UserName)
	}
	if len(paths) != 1 || paths[0] != "GET /api/v1/namespaces/wl-ns/secrets/wl-kubeconfig" {
		t.Fatalf("requetes inattendues : %v", paths)
	}
}
