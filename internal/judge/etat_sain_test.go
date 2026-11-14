package judge

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestEtatSainInchange : sur un instantane REEL de l'etat sain (faits + history nettoye des derives resolues), les regles
// corrigees donnent EXACTEMENT les memes derives, les memes verdicts (un par derive, tous Confirmed) et les memes dates
// firstDetectedAt que les regles d'avant la correction. Variables : SYLVATWIN_SAIN_DIR (facts.nt, history.nt), SYLVATWIN_POC_OLD
// (regles avant), SYLVATWIN_POC (regles apres), OXIGRAPH_BIN.
func TestEtatSainInchange(t *testing.T) {
	dir, old, nw, bin := os.Getenv("SYLVATWIN_SAIN_DIR"), os.Getenv("SYLVATWIN_POC_OLD"), os.Getenv("SYLVATWIN_POC"), os.Getenv("OXIGRAPH_BIN")
	if dir == "" || old == "" || nw == "" || bin == "" {
		t.Skip("SYLVATWIN_SAIN_DIR / SYLVATWIN_POC_OLD / SYLVATWIN_POC / OXIGRAPH_BIN non definis")
	}
	facts := mustReadFile(t, filepath.Join(dir, "facts.nt"))
	run := func(poc string) (drifts, verdicts, firsts []string) {
		e, stop := startOxigraph(t, bin)
		defer stop()
		hist := filepath.Join(t.TempDir(), "history.nt")
		if err := os.WriteFile(hist, mustReadFile(t, filepath.Join(dir, "history.nt")), 0o644); err != nil {
			t.Fatal(err)
		}
		p := &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600, HistoryFile: hist, TBoxFile: filepath.Join(poc, "sylvatwin-core-v1.ttl")}
		now := time.Now().UTC()
		for i := 0; i < 3; i++ { // 3 cycles : l'etat sain doit etre un point fixe
			res, err := p.Run(context.Background(), facts, now.Add(time.Duration(i)*10*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if i < 2 {
				continue
			}
			for _, l := range strings.Split(string(res.Drifts), "\n") {
				if s, pr, o, ok := splitNT(l); ok && strings.HasSuffix(pr, "22-rdf-syntax-ns#type") && strings.Contains(o, "core#") && !strings.Contains(s, "/FunctionImpactDrift/") {
					drifts = append(drifts, strings.TrimPrefix(o, "https://w3id.org/sylvatwin/core#")+" "+s)
				}
				if s, pr, o, ok := splitNT(l); ok && strings.HasSuffix(pr, "22-rdf-syntax-ns#type") && strings.HasSuffix(o, "FunctionImpactDrift") {
					drifts = append(drifts, "FunctionImpactDrift "+s)
				}
			}
			for _, l := range strings.Split(string(res.Verdicts), "\n") {
				if s, pr, o, ok := splitNT(l); ok && strings.HasSuffix(pr, "22-rdf-syntax-ns#type") && strings.Contains(o, "core#") {
					verdicts = append(verdicts, s+" "+strings.TrimPrefix(o, "https://w3id.org/sylvatwin/core#"))
				}
			}
		}
		b, _ := e.Select(context.Background(), `SELECT ?d ?f WHERE { GRAPH <`+GraphHistory+`> { ?d <https://w3id.org/sylvatwin/core#firstDetectedAt> ?f } }`)
		for _, part := range strings.Split(string(b), `"d":`)[1:] {
			d := strings.SplitN(strings.SplitN(part, `"value":"`, 2)[1], `"`, 2)[0]
			f := strings.SplitN(strings.SplitN(part, `"f":`, 2)[1], `"value":"`, 2)[1]
			firsts = append(firsts, d+" "+strings.SplitN(f, `"`, 2)[0])
		}
		sort.Strings(drifts)
		sort.Strings(verdicts)
		sort.Strings(firsts)
		return
	}
	d0, v0, f0 := run(old)
	d1, v1, f1 := run(nw)
	count := func(ls []string, pre string) (n int) {
		for _, l := range ls {
			if strings.HasPrefix(l, pre) {
				n++
			}
		}
		return
	}
	t.Logf("apres correction : StaleConfigDrift=%d FunctionImpactDrift=%d verdicts=%d (Confirmed=%d) firstDetectedAt=%d", count(d1, "StaleConfigDrift"), count(d1, "FunctionImpactDrift"), len(v1), count(v1, "")-count(vNot(v1), ""), len(f1))
	if count(d1, "StaleConfigDrift") != 5 || count(d1, "FunctionImpactDrift") != 11 || len(d1) != 16 {
		t.Errorf("etat sain attendu 5 StaleConfigDrift + 11 FunctionImpactDrift, obtenu %d / %d (total %d)", count(d1, "StaleConfigDrift"), count(d1, "FunctionImpactDrift"), len(d1))
	}
	if len(v1) != 16 || len(vNot(v1)) != 0 {
		t.Errorf("verdicts : attendu 16, tous Confirmed ; obtenu %d dont %d non Confirmed", len(v1), len(vNot(v1)))
	}
	if len(f1) != 16 {
		t.Errorf("firstDetectedAt : attendu 16 (un par derive), obtenu %d", len(f1))
	}
	for _, c := range [][3][]string{{d0, d1, {"derives"}}, {v0, v1, {"verdicts"}}, {f0, f1, {"firstDetectedAt"}}} {
		if strings.Join(c[0], "\n") != strings.Join(c[1], "\n") {
			t.Errorf("%s differents avant/apres correction (%d vs %d)", c[2][0], len(c[0]), len(c[1]))
		}
	}
}

func vNot(v []string) (out []string) {
	for _, l := range v {
		if !strings.HasSuffix(l, " ConfirmedViolation") {
			out = append(out, l)
		}
	}
	return
}
