package judge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// runLegacy : l'ALGORITHME D'AVANT A4, conserve ici comme reference figee pour
// le test de non-regression. A chaque passe, il recopie l'ABox (facts) dans le
// graphe par defaut (DROP, puis ADD), puis les derives deja trouvees (PUT puis
// ADD), et evalue les regles sur ce graphe par defaut. Ne pas modifier : c'est
// l'etalon. Le code de production (Pipeline.Run) ne fait plus aucune copie.
func (p *Pipeline) runLegacy(ctx context.Context, facts []byte, now time.Time) (*Result, error) {
	res := &Result{Timings: map[string]time.Duration{}}
	nowLit := now.UTC().Format("2006-01-02T15:04:05Z")
	seconds, err := os.ReadFile(filepath.Join(p.RulesDir, "verdict", "seconds.sparql"))
	if err != nil {
		return nil, err
	}
	prep := func(q string) string { return Prepare(q, nowLit, p.Validity, strings.TrimSpace(string(seconds))) }

	t0 := time.Now()
	if err := p.E.PutGraph(ctx, GraphFacts, facts); err != nil {
		return nil, err
	}
	res.Timings["load"] = time.Since(t0)
	files, _ := filepath.Glob(filepath.Join(p.RulesDir, "derives", "*.rq"))
	sort.Strings(files)
	pass := func(prefixes ...string) []string {
		var out []string
		for _, f := range files {
			b := filepath.Base(f)
			for _, pf := range prefixes {
				if strings.HasPrefix(b, pf) {
					out = append(out, f)
				}
			}
		}
		return out
	}
	var drifts bytes.Buffer
	for i, group := range [][]string{pass("0"), pass("10-"), pass("11-")} {
		t := time.Now()
		if err := p.E.Update(ctx, "DROP SILENT DEFAULT"); err != nil {
			return nil, err
		}
		if err := p.E.Update(ctx, "ADD <"+GraphFacts+"> TO DEFAULT"); err != nil {
			return nil, err
		}
		if drifts.Len() > 0 {
			if err := p.E.PutGraph(ctx, GraphDrifts, drifts.Bytes()); err != nil {
				return nil, err
			}
			if err := p.E.Update(ctx, "ADD <"+GraphDrifts+"> TO DEFAULT"); err != nil {
				return nil, err
			}
		}
		for _, f := range group {
			q, _ := os.ReadFile(f)
			out, err := p.E.Construct(ctx, prep(string(q)))
			if err != nil {
				return nil, fmt.Errorf("%s : %w", filepath.Base(f), err)
			}
			drifts.Write(out)
		}
		res.Timings[fmt.Sprintf("passe%d", i+1)] = time.Since(t)
	}
	if err := p.E.Update(ctx, "DROP SILENT DEFAULT"); err != nil {
		return nil, err
	}
	if err := p.E.PutGraph(ctx, GraphDrifts, drifts.Bytes()); err != nil {
		return nil, err
	}
	res.Drifts = drifts.Bytes()
	t := time.Now()
	lc, _ := filepath.Glob(filepath.Join(p.RulesDir, "lifecycle", "*.ru"))
	sort.Strings(lc)
	for _, f := range lc {
		u, _ := os.ReadFile(f)
		if err := p.E.Update(ctx, prep(string(u))); err != nil {
			return nil, err
		}
	}
	res.Timings["lifecycle"] = time.Since(t)
	t = time.Now()
	vf, _ := filepath.Glob(filepath.Join(p.RulesDir, "verdict", "*.rq"))
	sort.Strings(vf)
	var verdicts bytes.Buffer
	for _, f := range vf {
		q, _ := os.ReadFile(f)
		out, err := p.E.Construct(ctx, prep(string(q)))
		if err != nil {
			return nil, err
		}
		verdicts.Write(out)
	}
	res.Timings["verdict"] = time.Since(t)
	res.Verdicts = verdicts.Bytes()
	return res, nil
}

var detectedAtRe = regexp.MustCompile(`<https://w3id.org/sylvatwin/core#detectedAt>`)

// canon : ensemble de triplets N-Triples, trie. detectedAt (= NOW() de la regle,
// instant d'execution, different d'un lancement a l'autre) est retire ; tout le
// reste doit etre IDENTIQUE.
func canon(nt []byte) []string {
	var out []string
	for _, l := range strings.Split(string(nt), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || detectedAtRe.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

func diffSets(a, b []string) (onlyA, onlyB []string) {
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for k, v := range m {
		if v > 0 {
			onlyA = append(onlyA, k)
		} else if v < 0 {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return
}

func head(ss []string, n int) string {
	if len(ss) > n {
		ss = ss[:n]
	}
	return strings.Join(ss, "\n")
}

// TestNonRegressionA4Archive : sur les faits ARCHIVES (campagne du 09/11), deux
// serveurs Oxigraph recoivent la meme suite de cycles, l'un par l'ancien
// algorithme (copies), l'autre par le nouveau. A chaque cycle : derives,
// verdicts ET historique doivent etre identiques (a detectedAt pres).
//
//	SYLVATWIN_ARCHIVE      : dossier des faits extraits (<scenario>/<rep>/cycles/cycle-NNNN/facts.nt)
//	SYLVATWIN_ARCHIVE_REPS : repetitions retenues par scenario (defaut "1")
func TestNonRegressionA4Archive(t *testing.T) {
	root := os.Getenv("SYLVATWIN_ARCHIVE")
	bin := os.Getenv("OXIGRAPH_BIN")
	poc := os.Getenv("SYLVATWIN_POC")
	if root == "" || bin == "" || poc == "" {
		t.Skip("SYLVATWIN_ARCHIVE / OXIGRAPH_BIN / SYLVATWIN_POC non definis")
	}
	reps := strings.Split(envOr("SYLVATWIN_ARCHIVE_REPS", "1"), ",")
	eOld, stopOld := startOxigraph(t, bin)
	defer stopOld()
	eNew, stopNew := startOxigraph(t, bin)
	defer stopNew()
	ctx := context.Background()
	rules := filepath.Join(poc, "rules")
	pOld := &Pipeline{E: eOld, RulesDir: rules, Validity: 600}
	pNew := &Pipeline{E: eNew, RulesDir: rules, Validity: 600}

	scen, _ := filepath.Glob(filepath.Join(root, "*"))
	sort.Strings(scen)
	total, nonEmpty := 0, 0
	for _, sd := range scen {
		for _, rep := range reps {
			cyc, _ := filepath.Glob(filepath.Join(sd, rep, "cycles", "cycle-*"))
			if len(cyc) == 0 {
				continue
			}
			sort.Strings(cyc)
			// historique vide au debut de chaque suite
			for _, e := range []*Endpoint{eOld, eNew} {
				for _, g := range []string{GraphHistory, GraphDrifts, GraphFacts} {
					if err := e.DropGraph(ctx, g); err != nil {
						t.Fatal(err)
					}
				}
			}
			base := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
			for i, c := range cyc {
				facts, err := os.ReadFile(filepath.Join(c, "facts.nt"))
				if err != nil {
					t.Fatal(err)
				}
				now := base.Add(time.Duration(i) * 40 * time.Second)
				ro, err := pOld.runLegacy(ctx, facts, now)
				if err != nil {
					t.Fatalf("%s/%s ancien : %v", filepath.Base(sd), rep, err)
				}
				rn, err := pNew.Run(ctx, facts, now)
				if err != nil {
					t.Fatalf("%s/%s nouveau : %v", filepath.Base(sd), rep, err)
				}
				id := fmt.Sprintf("%s/%s/%s", filepath.Base(sd), rep, filepath.Base(c))
				check := func(what string, a, b []byte) {
					ca, cb := canon(a), canon(b)
					if x, y := diffSets(ca, cb); len(x)+len(y) > 0 {
						t.Errorf("%s : %s DIFFERENTS (ancien seul : %d, nouveau seul : %d)\n-- ancien seul\n%s\n-- nouveau seul\n%s",
							id, what, len(x), len(y), head(x, 5), head(y, 5))
					}
				}
				check("derives", ro.Drifts, rn.Drifts)
				check("verdicts", ro.Verdicts, rn.Verdicts)
				ho, _ := eOld.GetGraph(ctx, GraphHistory)
				hn, _ := eNew.GetGraph(ctx, GraphHistory)
				check("historique", ho, hn)
				total++
				if len(canon(rn.Drifts)) > 0 {
					nonEmpty++
				}
			}
		}
	}
	if total == 0 {
		t.Fatal("aucun cycle archive trouve dans " + root)
	}
	t.Logf("non-regression A4 : %d cycles compares (derives + verdicts + historique), dont %d avec au moins une derive", total, nonEmpty)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// TestMesureA4 : temps de chaque etape du juge, ancien algorithme contre
// nouveau, sur le MEME ABox (SYLVATWIN_FACTS). N cycles chacun (SYLVATWIN_N,
// defaut 10), medianes. Sur des serveurs Oxigraph neufs et separes.
func TestMesureA4(t *testing.T) {
	facts := os.Getenv("SYLVATWIN_FACTS")
	bin := os.Getenv("OXIGRAPH_BIN")
	poc := os.Getenv("SYLVATWIN_POC")
	if facts == "" || bin == "" || poc == "" {
		t.Skip("SYLVATWIN_FACTS / OXIGRAPH_BIN / SYLVATWIN_POC non definis")
	}
	n, _ := strconv.Atoi(envOr("SYLVATWIN_N", "10"))
	nt, err := os.ReadFile(facts)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rules := filepath.Join(poc, "rules")
	steps := []string{"load", "passe1", "passe2", "passe3", "lifecycle", "verdict"}
	run := func(legacy bool) map[string][]float64 {
		e, stop := startOxigraph(t, bin)
		defer stop()
		p := &Pipeline{E: e, RulesDir: rules, Validity: 600}
		out := map[string][]float64{}
		for i := 0; i < n+1; i++ { // le 1er cycle (base vide) est ecarte
			now := time.Now().UTC()
			t0 := time.Now()
			var r *Result
			var err error
			if legacy {
				r, err = p.runLegacy(ctx, nt, now)
			} else {
				r, err = p.Run(ctx, nt, now)
			}
			if err != nil {
				t.Fatal(err)
			}
			tot := time.Since(t0)
			if i == 0 {
				continue
			}
			for _, s := range steps {
				out[s] = append(out[s], float64(r.Timings[s].Milliseconds()))
			}
			out["TOTAL"] = append(out["TOTAL"], float64(tot.Milliseconds()))
		}
		return out
	}
	med := func(v []float64) float64 {
		s := append([]float64{}, v...)
		sort.Float64s(s)
		return s[len(s)/2]
	}
	oldM, newM := run(true), run(false)
	var b strings.Builder
	fmt.Fprintf(&b, "\n%-10s %12s %12s   (mediane de %d cycles, ms)\n", "etape", "AVANT", "APRES", n)
	for _, s := range append(steps, "TOTAL") {
		fmt.Fprintf(&b, "%-10s %12.0f %12.0f\n", s, med(oldM[s]), med(newM[s]))
	}
	t.Log(b.String())
}

// TestDisqueA4 : taille disque de la base Oxigraph apres N cycles sur le MEME
// ABox, ancien algorithme contre nouveau, sur des dossiers neufs
// (SYLVATWIN_FACTS, SYLVATWIN_N, defaut 100). Pas d'assertion : une mesure.
func TestDisqueA4(t *testing.T) {
	facts := os.Getenv("SYLVATWIN_FACTS")
	bin := os.Getenv("OXIGRAPH_BIN")
	poc := os.Getenv("SYLVATWIN_POC")
	if facts == "" || bin == "" || poc == "" {
		t.Skip("SYLVATWIN_FACTS / OXIGRAPH_BIN / SYLVATWIN_POC non definis")
	}
	n, _ := strconv.Atoi(envOr("SYLVATWIN_N", "100"))
	nt, _ := os.ReadFile(facts)
	ctx := context.Background()
	rules := filepath.Join(poc, "rules")
	du := func(dir string) int64 {
		var sum int64
		_ = filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				sum += fi.Size()
			}
			return nil
		})
		return sum
	}
	run := func(legacy bool) (start, end int64, dur time.Duration) {
		dir := t.TempDir()
		e, stop := startOxigraphAt(t, bin, dir)
		defer stop()
		p := &Pipeline{E: e, RulesDir: rules, Validity: 600}
		t0 := time.Now()
		for i := 0; i < n; i++ {
			var err error
			if legacy {
				_, err = p.runLegacy(ctx, nt, time.Now().UTC())
			} else {
				_, err = p.Run(ctx, nt, time.Now().UTC())
			}
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				start = du(dir)
			}
		}
		return start, du(dir), time.Since(t0)
	}
	so, eo, do := run(true)
	sn, en, dn := run(false)
	mb := func(b int64) float64 { return float64(b) / 1e6 }
	t.Logf("\napres %d cycles, meme ABox (%d triplets) :\n  AVANT (copies) : apres 1 cycle %.0f Mo, apres %d cycles %.0f Mo (+%.0f Mo), duree %s\n  APRES (union)  : apres 1 cycle %.0f Mo, apres %d cycles %.0f Mo (+%.0f Mo), duree %s",
		n, strings.Count(string(nt), "\n"), mb(so), n, mb(eo), mb(eo-so), do.Round(time.Second), mb(sn), n, mb(en), mb(en-sn), dn.Round(time.Second))
}
