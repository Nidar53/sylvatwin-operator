package judge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// A4bis : Oxigraph en memoire perd tout au redemarrage, sauf <history> recharge
// depuis le fichier : l'age d'une derive existante est conserve.
func TestHistoriqueSurvitAuRedemarrage(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	bin := os.Getenv("OXIGRAPH_BIN")
	if poc == "" || bin == "" {
		t.Skip("SYLVATWIN_POC / OXIGRAPH_BIN non definis")
	}
	ref := t.TempDir()
	if out, err := exec.Command("python3", filepath.Join(poc, "tests", "export_reference.py"), ref).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	facts := mustReadFile(t, filepath.Join(ref, "facts-t60.nt"))
	hist := filepath.Join(t.TempDir(), "history.nt")
	t0 := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	ages := func(res *Result) map[string]string {
		m := map[string]string{}
		for _, l := range strings.Split(string(res.Verdicts), "\n") {
			if s, pr, o, ok := splitNT(l); ok && strings.HasSuffix(pr, "#driftAge") {
				m[s] = strings.SplitN(strings.Trim(o, `"`), `"`, 2)[0]
			}
		}
		return m
	}
	ctx := context.Background()
	e1, stop1 := startOxigraph(t, bin)
	defer stop1()
	p1 := &Pipeline{E: e1, RulesDir: filepath.Join(poc, "rules"), Validity: 600, HistoryFile: hist}
	if _, err := p1.Run(ctx, facts, t0); err != nil {
		t.Fatal(err)
	}
	r1, err := p1.Run(ctx, facts, t0.Add(100*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	stop1() // "redemarrage" : un Oxigraph tout neuf, vide
	e2, stop2 := startOxigraph(t, bin)
	defer stop2()
	p2 := &Pipeline{E: e2, RulesDir: filepath.Join(poc, "rules"), Validity: 600, HistoryFile: hist}
	r2, err := p2.Run(ctx, facts, t0.Add(200*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	a1, a2 := ages(r1), ages(r2)
	if len(a1) == 0 {
		t.Fatal("aucune derive dans le scenario de reference")
	}
	for d, age1 := range a1 {
		// 100 s plus tard : l'age a grandi de 100 s, il n'est pas reparti de 0
		if age1 != "100" || a2[d] != "200" {
			t.Errorf("%s : age avant redemarrage %q (attendu 100), apres %q (attendu 200)", d, age1, a2[d])
		}
	}
}

// R2 : avec default-graph-uri, une regle de rules/derives qui contiendrait une
// clause GRAPH ne verrait plus rien, EN SILENCE (GRAPH <g> {...} cherche dans le
// graphe nomme <g>, et le parametre ne change que le graphe PAR DEFAUT). Les
// regles de derives/ doivent donc etre ecrites sans GRAPH.
func TestDerivesSansClauseGRAPH(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	files, _ := filepath.Glob(filepath.Join(poc, "rules", "derives", "*.rq"))
	if len(files) == 0 {
		t.Skipf("aucune regle dans %s", poc)
	}
	re := regexp.MustCompile(`(?i)\bGRAPH\b`)
	for _, f := range files {
		for i, l := range strings.Split(string(mustReadFile(t, f)), "\n") {
			if k := strings.Index(l, "#"); k >= 0 {
				l = l[:k] // les commentaires peuvent en parler
			}
			if re.MatchString(l) {
				t.Errorf("%s:%d contient GRAPH : interdit dans rules/derives. Le juge evalue ces regles sur l'union des graphes via default-graph-uri (A4, sans recopie) ; une clause GRAPH <g> ne regarde que le graphe nomme <g>, et la regle ne verrait plus rien, sans erreur. Ecrire le motif sans GRAPH.", filepath.Base(f), i+1)
			}
		}
	}
}
