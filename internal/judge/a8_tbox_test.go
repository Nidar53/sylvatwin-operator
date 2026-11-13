package judge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// La TBox est chargee dans <tbox>, rechargee si le serveur l'a perdue (Oxigraph en
// memoire), et refusee si elle est trop ancienne pour les regles.
func TestTBoxChargeeRechargeeEtVersionnee(t *testing.T) {
	poc, bin := os.Getenv("SYLVATWIN_POC"), os.Getenv("OXIGRAPH_BIN")
	if poc == "" || bin == "" {
		t.Skip("SYLVATWIN_POC / OXIGRAPH_BIN non definis")
	}
	e, stop := startOxigraph(t, bin)
	defer stop()
	ctx := context.Background()
	p := &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600}
	has := func() bool {
		out, err := e.Select(ctx, `ASK { GRAPH <`+GraphTBox+`> { <https://w3id.org/sylvatwin/core#UnattendedDrift> <http://www.w3.org/2000/01/rdf-schema#subClassOf> <https://w3id.org/sylvatwin/core#RootDrift> } }`)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(out), "true")
	}
	if err := p.ensureTBox(ctx); err != nil || !has() {
		t.Fatalf("chargement initial : %v (present=%v)", err, has())
	}
	if err := e.DropGraph(ctx, GraphTBox); err != nil { // le serveur "redemarre"
		t.Fatal(err)
	}
	if has() {
		t.Fatal("graphe tbox encore present apres DROP")
	}
	if err := p.ensureTBox(ctx); err != nil || !has() {
		t.Fatalf("rechargement : %v (present=%v)", err, has())
	}
	old := filepath.Join(t.TempDir(), "old.ttl")
	if err := os.WriteFile(old, []byte(`@prefix owl: <http://www.w3.org/2002/07/owl#> . <https://w3id.org/sylvatwin/core> owl:versionInfo "1.3.0" .`), 0o644); err != nil {
		t.Fatal(err)
	}
	p.TBoxFile = old
	if err := p.ensureTBox(ctx); err == nil || !strings.Contains(err.Error(), "au moins") {
		t.Fatalf("une TBox 1.3.0 doit etre refusee, obtenu %v", err)
	}
}
