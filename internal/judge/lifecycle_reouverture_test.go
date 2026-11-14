package judge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestConformiteReouverture : les cas de reouverture / horizon recalcule (correction du 14/11/2026,
// docs/DIAGNOSTIC-SUSPENSION-2026-11-14.md) donnent, sous Oxigraph, EXACTEMENT la sequence calculee par rdflib
// (tests/export_reference.py : reouverture-sequence.tsv) : nombre de firstDetectedAt, horizons, verdicts de la derive
// GitOpsSyncDrift de la Kustomization a chaque etape. Variables : SYLVATWIN_POC (depot poc), OXIGRAPH_BIN ; python3 + rdflib (PYTHONPATH).
func TestConformiteReouverture(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	bin := os.Getenv("OXIGRAPH_BIN")
	if bin == "" {
		t.Skip("OXIGRAPH_BIN non defini")
	}
	ref := t.TempDir()
	if out, err := exec.Command("python3", filepath.Join(poc, "tests", "export_reference.py"), ref).CombinedOutput(); err != nil {
		t.Fatalf("reference rdflib : %v\n%s", err, out)
	}
	rows := strings.Split(strings.TrimSpace(string(mustReadFile(t, filepath.Join(ref, "reouverture-sequence.tsv")))), "\n")
	e, stop := startOxigraph(t, bin)
	defer stop()
	ctx := context.Background()
	const gsd = "https://w3id.org/sylvatwin/drift/GitOpsSyncDrift/management-cluster/sylvatwin-flux/Kustomization.kustomize.toolkit.fluxcd.io/sylvatwin-5gc"
	var cur string
	var p *Pipeline
	var got, want []string
	for _, r := range rows {
		f := strings.Split(r, "\t")
		if f[0] != cur {
			cur = f[0]
			for _, g := range []string{GraphHistory, GraphDrifts, GraphFacts} {
				if err := e.DropGraph(ctx, g); err != nil {
					t.Fatal(err)
				}
			}
			p = &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600}
		}
		sec, _ := strconv.Atoi(f[1])
		facts := mustReadFile(t, filepath.Join(ref, fmt.Sprintf("reouv-%s-t%d.nt", f[2], sec)))
		res, err := p.Run(ctx, facts, time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC).Add(time.Duration(sec)*time.Second))
		if err != nil {
			t.Fatalf("%s t=%d : %v", f[0], sec, err)
		}
		sel := func(q string) []string {
			b, err := e.Select(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			var out []string
			for _, part := range strings.Split(string(b), `"value":"`)[1:] {
				out = append(out, strings.SplitN(part, `"`, 2)[0])
			}
			return out
		}
		nf := sel(`SELECT (COUNT(*) AS ?n) WHERE { GRAPH <` + GraphHistory + `> { <` + gsd + `> <https://w3id.org/sylvatwin/core#firstDetectedAt> ?f } }`)[0]
		hz := sel(`SELECT ?h WHERE { GRAPH <` + GraphHistory + `> { <` + gsd + `> <https://w3id.org/sylvatwin/core#convergenceHorizon> ?h } }`)
		sort.Strings(hz)
		for i := range hz {
			x, _ := strconv.ParseFloat(hz[i], 64)
			hz[i] = strconv.FormatFloat(x, 'f', 1, 64)
		}
		hzs := strings.Join(hz, ",")
		if hzs == "" {
			hzs = "-"
		}
		vt := map[string]map[string]bool{}
		for _, l := range strings.Split(string(res.Verdicts), "\n") {
			s, pr, o, ok := splitNT(l)
			if ok && pr == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" && strings.Contains(s, "/verdict/GitOpsSyncDrift/") {
				if vt[s] == nil {
					vt[s] = map[string]bool{}
				}
				vt[s][strings.TrimPrefix(o, "https://w3id.org/sylvatwin/core#")] = true
			}
		}
		var vs []string
		for _, m := range vt {
			var c []string
			for k := range m {
				c = append(c, k)
			}
			sort.Strings(c)
			vs = append(vs, strings.Join(c, "+"))
		}
		v := strings.Join(vs, ";")
		if v == "" {
			v = "-"
		}
		got = append(got, strings.Join([]string{f[0], f[1], f[2], nf, hzs, v}, "\t"))
		want = append(want, r)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sequences differentes\n--- rdflib\n%s\n--- oxigraph\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	t.Logf("reouverture : %d etapes identiques rdflib / oxigraph", len(got))
	for _, l := range got {
		if f := strings.Split(l, "\t"); f[3] != "1" {
			t.Errorf("plus d'un firstDetectedAt : %s", l)
		}
	}
}
