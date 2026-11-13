package judge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// A5 : une derive n'est "inconnue" que si un cluster de SES ressources est
// perime ou non observe. Deux clusters : A (temoin perime), B (temoin frais).
func TestVerdictParCluster_DeuxClusters(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	bin := os.Getenv("OXIGRAPH_BIN")
	if poc == "" || bin == "" {
		t.Skip("SYLVATWIN_POC / OXIGRAPH_BIN non definis")
	}
	e, stop := startOxigraph(t, bin)
	defer stop()
	ctx := context.Background()

	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	ts := func(d time.Duration) string { return now.Add(-d).Format("2006-01-02T15:04:05Z") }
	const c = "https://w3id.org/sylvatwin/core#"
	const r = "https://w3id.org/sylvatwin/res/"
	const dr = "https://w3id.org/sylvatwin/drift/"
	dt := func(s string) string { return `"` + s + `"^^<http://www.w3.org/2001/XMLSchema#dateTime>` }
	var facts, drifts, hist strings.Builder
	f := func(s, p, o string) { fmt.Fprintf(&facts, "<%s> <%s%s> %s .\n", s, c, p, o) }
	cl := func(name string, witnessAge time.Duration) {
		fmt.Fprintf(&facts, "<%s%s> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <%sKubernetesCluster> .\n", r, name, c)
		f(r+name, "observationStatus", `"ok"`)
		f(r+name, "lastSyncTime", dt(ts(5*time.Second)))
		f(r+name, "lastWitnessRenewTime", dt(ts(witnessAge)))
		f(r+name, "freshnessHorizon", `"30.0"^^<http://www.w3.org/2001/XMLSchema#decimal>`)
	}
	cl("A", 100*time.Second) // perime : 100 s > 30 s
	cl("B", 5*time.Second)   // frais
	in := func(res, cluster string) { f(r+res, "resourceInCluster", "<"+r+cluster+">") }
	in("a1", "A")
	in("b1", "B")
	in("depA", "A")
	in("z1", "Z") // cluster Z : aucun fait KubernetesCluster (non observe)
	f(r+"nf", "realizedBy", "<"+r+"depA>")
	aff := func(d string, res ...string) {
		for _, x := range res {
			fmt.Fprintf(&drifts, "<%s%s> <%saffects> <%s%s> .\n", dr, d, c, r, x)
		}
		fmt.Fprintf(&drifts, "<%s%s> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <%sSpecDrift> .\n", dr, d, c)
		fmt.Fprintf(&hist, "<%s%s> <%sfirstDetectedAt> %s .\n", dr, d, c, dt(ts(1000*time.Second)))
	}
	cause := func(d, o string) {
		fmt.Fprintf(&drifts, "<%s%s> <%scausedBy> <%s%s> .\n", dr, d, c, dr, o)
		fmt.Fprintf(&drifts, "<%s%s> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <%sFunctionImpactDrift> .\n", dr, d, c)
		fmt.Fprintf(&hist, "<%s%s> <%sfirstDetectedAt> %s .\n", dr, d, c, dt(ts(1000*time.Second)))
	}
	aff("dA", "a1")        // cluster A perime      -> Unknown
	aff("dB", "b1")        // cluster B frais       -> Confirmed
	aff("dNF", "nf")       // NF realisee dans A    -> Unknown
	aff("dPLMN")           // aucune ressource      -> Confirmed
	aff("dZ", "z1")        // cluster non observe   -> Unknown
	aff("dAB", "a1", "b1") // touche A et B         -> Unknown
	cause("iB", "dB")      // impact de cause en B  -> Confirmed
	cause("iA", "dA")      // impact de cause en A  -> Unknown
	// derive ouverte mais non re-derivee ce cycle (garde P3) : tous les clusters
	// comptent, A est perime -> Unknown
	fmt.Fprintf(&hist, "<%sdP3> <%sfirstDetectedAt> %s .\n", dr, c, dt(ts(1000*time.Second)))

	for g, d := range map[string]string{GraphFacts: facts.String(), GraphDrifts: drifts.String(), GraphHistory: hist.String()} {
		if err := e.PutGraph(ctx, g, []byte(d)); err != nil {
			t.Fatal(err)
		}
	}
	q, err := os.ReadFile(filepath.Join(poc, "rules", "verdict", "01-verdict.rq"))
	if err != nil {
		t.Fatal(err)
	}
	sec, _ := os.ReadFile(filepath.Join(poc, "rules", "verdict", "seconds.sparql"))
	out, err := e.Construct(ctx, Prepare(string(q), now.Format("2006-01-02T15:04:05Z"), 600, strings.TrimSpace(string(sec))))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range strings.Split(string(out), "\n") {
		s, pr, o, ok := splitNT(l)
		if ok && strings.HasSuffix(pr, "22-rdf-syntax-ns#type") {
			got[strings.TrimPrefix(s, "https://w3id.org/sylvatwin/verdict/")] = strings.TrimPrefix(o, c)
		}
	}
	want := map[string]string{"dA": "UnknownVerdict", "dB": "ConfirmedViolation", "dNF": "UnknownVerdict",
		"dPLMN": "ConfirmedViolation", "dZ": "UnknownVerdict", "dAB": "UnknownVerdict",
		"iB": "ConfirmedViolation", "iA": "UnknownVerdict", "dP3": "UnknownVerdict"}
	var keys []string
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if got[k] != want[k] {
			t.Errorf("%s : %q, attendu %q (tous : %v)", k, got[k], want[k], got)
		}
	}
}
