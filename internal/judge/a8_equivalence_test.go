package judge

import (
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

// TestA8Equivalence : memes faits archives (campagne du 09/11), deux jeux de
// regles : l'ancien (listes FILTER IN) et le nouveau (hierarchie de la TBox).
// Derives, verdicts et historique doivent etre identiques a chaque cycle.
//
//	SYLVATWIN_POC_OLD : copie des regles AVANT A8 (git archive de l'ancien commit)
//	SYLVATWIN_POC     : depot courant (regles + TBox 1.4.0)
//	SYLVATWIN_ARCHIVE : faits archives
func TestA8Equivalence(t *testing.T) {
	old, root := os.Getenv("SYLVATWIN_POC_OLD"), os.Getenv("SYLVATWIN_ARCHIVE")
	bin, poc := os.Getenv("OXIGRAPH_BIN"), os.Getenv("SYLVATWIN_POC")
	if old == "" || root == "" || bin == "" || poc == "" {
		t.Skip("SYLVATWIN_POC_OLD / SYLVATWIN_ARCHIVE / OXIGRAPH_BIN / SYLVATWIN_POC non definis")
	}
	eOld, stopOld := startOxigraph(t, bin)
	defer stopOld()
	eNew, stopNew := startOxigraph(t, bin)
	defer stopNew()
	ctx := context.Background()
	tbox := filepath.Join(poc, "sylvatwin-core-v1.ttl")
	pOld := &Pipeline{E: eOld, RulesDir: filepath.Join(old, "rules"), Validity: 600, TBoxFile: tbox}
	pNew := &Pipeline{E: eNew, RulesDir: filepath.Join(poc, "rules"), Validity: 600, TBoxFile: tbox}
	scen, _ := filepath.Glob(filepath.Join(root, "*"))
	sort.Strings(scen)
	total, withDrift, withImpact := 0, 0, 0
	for _, sd := range scen {
		cyc, _ := filepath.Glob(filepath.Join(sd, envOr("SYLVATWIN_ARCHIVE_REP", "1"), "cycles", "cycle-*"))
		sort.Strings(cyc)
		if re := os.Getenv("SYLVATWIN_ARCHIVE_ONLY"); re != "" && !regexp.MustCompile(re).MatchString(filepath.Base(sd)) {
			continue
		}
		if step, _ := strconv.Atoi(os.Getenv("SYLVATWIN_ARCHIVE_STEP")); step > 1 {
			var keep []string
			for i, c := range cyc {
				if i%step == 0 {
					keep = append(keep, c)
				}
			}
			cyc = keep
		}
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
			ro, err := pOld.Run(ctx, facts, now)
			if err != nil {
				t.Fatal(err)
			}
			rn, err := pNew.Run(ctx, facts, now)
			if err != nil {
				t.Fatal(err)
			}
			id := fmt.Sprintf("%s/%s", filepath.Base(sd), filepath.Base(c))
			check := func(what string, a, b []byte) {
				if x, y := diffSets(canon(a), canon(b)); len(x)+len(y) > 0 {
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
				withDrift++
			}
			if containsStr(rn.Drifts, "FunctionImpactDrift") {
				withImpact++
			}
		}
	}
	if total == 0 {
		t.Fatal("aucun cycle archive")
	}
	t.Logf("A8 : %d cycles compares, %d avec derive, %d avec impact", total, withDrift, withImpact)
}

func containsStr(b []byte, s string) bool { return strings.Contains(string(b), s) }
