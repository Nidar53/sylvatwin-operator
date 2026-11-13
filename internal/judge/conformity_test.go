package judge

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// BARRIERE 2 : Oxigraph (via le protocole SPARQL 1.1) doit produire EXACTEMENT
// les memes derives que rdflib (moteur de reference de sylvatwin-poc) sur les
// variantes de tests/export_reference.py.
//
// Prerequis (sinon le test est ignore, avec la raison) :
//
//	SYLVATWIN_POC : depot sylvatwin-poc (rules/, tests/) ; defaut ../../../sylvatwin-poc
//	OXIGRAPH_BIN  : binaire oxigraph (v0.5.x) ; python3 + rdflib + owlrl.
func TestConformiteRdflibOxigraph(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	bin := os.Getenv("OXIGRAPH_BIN")
	if _, err := os.Stat(filepath.Join(poc, "tests", "export_reference.py")); err != nil {
		t.Skipf("sylvatwin-poc introuvable (%s)", poc)
	}
	if bin == "" {
		t.Skip("OXIGRAPH_BIN non defini")
	}
	ref := t.TempDir()
	cmd := exec.Command("python3", filepath.Join(poc, "tests", "export_reference.py"), ref)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reference rdflib : %v\n%s", err, out)
	}
	e, stop := startOxigraph(t, bin)
	defer stop()

	variants, _ := filepath.Glob(filepath.Join(ref, "*.facts.nt"))
	sort.Strings(variants)
	if len(variants) < 11 {
		t.Fatalf("seulement %d variantes", len(variants))
	}
	ctx := context.Background()
	p := &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600}
	for _, f := range variants {
		name := strings.TrimSuffix(filepath.Base(f), ".facts.nt")
		facts, _ := os.ReadFile(f)
		// chaque variante part d'un historique vide
		for _, g := range []string{GraphHistory, GraphDrifts, GraphFacts} {
			if err := e.DropGraph(ctx, g); err != nil {
				t.Fatal(err)
			}
		}
		res, err := p.Run(ctx, facts, time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		got := driftLines(t, res.Drifts)
		want := readLines(t, filepath.Join(ref, name+".drifts.tsv"))
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s : derives differentes\n--- rdflib (%d)\n%s\n--- oxigraph (%d)\n%s",
				name, len(want), strings.Join(want, "\n"), len(got), strings.Join(got, "\n"))
		} else {
			t.Logf("%-22s identique : %d lignes", name, len(got))
		}
	}
}

// driftLines : meme normalisation que export_reference.py.
func driftLines(t *testing.T, nt []byte) []string {
	t.Helper()
	type tr struct{ s, p, o string }
	var ts []tr
	sc := bufio.NewScanner(bytes.NewReader(nt))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" {
			continue
		}
		s, p, o, ok := splitNT(l)
		if !ok {
			t.Fatalf("ligne N-Triples illisible : %s", l)
		}
		ts = append(ts, tr{s, p, o})
	}
	const C = "https://w3id.org/sylvatwin/core#"
	val := func(s, p string) string {
		for _, x := range ts {
			if x.s == s && x.p == p {
				return x.o
			}
		}
		return "None"
	}
	set := map[string]bool{}
	for _, x := range ts {
		switch x.p {
		case "http://www.w3.org/1999/02/22-rdf-syntax-ns#type":
			row := strings.TrimPrefix(x.o, C) + "\t" + x.s
			if strings.HasSuffix(x.o, "FunctionImpactDrift") {
				row += "\t" + val(x.s, C+"affectsFunction") + "\t" + val(x.s, C+"impactPath")
			}
			set[row] = true
		case C + "affects":
			set["affects\t"+x.s+"\t"+x.o] = true
		}
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// splitNT : decoupe une ligne N-Triples en (sujet, predicat, objet) sans les
// chevrons ni le datatype (forme lexicale pour les litteraux).
func splitNT(l string) (string, string, string, bool) {
	l = strings.TrimSuffix(strings.TrimSpace(l), ".")
	l = strings.TrimSpace(l)
	term := func(s string) (string, string, bool) {
		s = strings.TrimLeft(s, " ")
		if strings.HasPrefix(s, "<") {
			i := strings.Index(s, ">")
			if i < 0 {
				return "", "", false
			}
			return s[1:i], s[i+1:], true
		}
		if strings.HasPrefix(s, "\"") {
			i := 1
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' {
					i++
				}
				i++
			}
			lex := s[1:i]
			rest := s[i+1:]
			if strings.HasPrefix(rest, "^^<") {
				j := strings.Index(rest, ">")
				rest = rest[j+1:]
			} else if strings.HasPrefix(rest, "@") {
				j := strings.IndexAny(rest, " \t")
				if j < 0 {
					j = len(rest)
				}
				rest = rest[j:]
			}
			return lex, rest, true
		}
		return "", "", false
	}
	s, r, ok := term(l)
	if !ok {
		return "", "", "", false
	}
	p, r, ok := term(r)
	if !ok {
		return "", "", "", false
	}
	o, _, ok := term(r)
	return s, p, o, ok
}

func readLines(t *testing.T, p string) []string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

// startOxigraph : instance jetable, base en memoire temporaire, 127.0.0.1 seulement.
func startOxigraph(t *testing.T, bin string) (*Endpoint, func()) {
	t.Helper()
	return startOxigraphAt(t, bin, t.TempDir())
}

// startOxigraphAt : comme startOxigraph, dans un dossier choisi (mesure disque).
func startOxigraphAt(t *testing.T, bin, dir string) (*Endpoint, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cmd := exec.Command(bin, "serve", "--location", dir, "--bind", fmt.Sprintf("127.0.0.1:%d", port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	e, err := NewEndpoint(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if _, err := e.Select(ctx, "ASK {}"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return e, func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }
}

func TestEndpointBoucleLocale(t *testing.T) {
	for _, bad := range []string{"http://10.0.0.1:7878", "http://example.org:7878", "http://172.30.0.3:7878"} {
		if _, err := NewEndpoint(bad); err == nil {
			t.Errorf("%s devrait etre refuse", bad)
		}
	}
	for _, ok := range []string{"http://127.0.0.1:7878", "http://localhost:7878", "http://[::1]:7878"} {
		if _, err := NewEndpoint(ok); err != nil {
			t.Errorf("%s devrait etre accepte : %v", ok, err)
		}
	}
}

func TestPrepare(t *testing.T) {
	q := Prepare(`BIND("__NOW__"^^xsd:dateTime AS ?now) BIND(__VALIDITY__ AS ?v) BIND(__SECONDS__(?now, ?first) AS ?age)`,
		"2026-11-09T12:00:00Z", 600, "(SECONDS($A)-SECONDS($B))")
	want := `BIND("2026-11-09T12:00:00Z"^^xsd:dateTime AS ?now) BIND(600.0 AS ?v) BIND((SECONDS(?now)-SECONDS(?first)) AS ?age)`
	if q != want {
		t.Fatalf("Prepare :\n%s\nattendu\n%s", q, want)
	}
}

// Conformite du CYCLE DE VIE et du VERDICT (S0-A, 3 cycles a t=60, 500, 600 s) :
// meme sequence (derive, verdict) et meme nombre de resolutions que rdflib.
func TestConformiteVerdict(t *testing.T) {
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	bin := os.Getenv("OXIGRAPH_BIN")
	if bin == "" {
		t.Skip("OXIGRAPH_BIN non defini")
	}
	if _, err := os.Stat(filepath.Join(poc, "tests", "export_reference.py")); err != nil {
		t.Skipf("sylvatwin-poc introuvable (%s)", poc)
	}
	ref := t.TempDir()
	if out, err := exec.Command("python3", filepath.Join(poc, "tests", "export_reference.py"), ref).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	e, stop := startOxigraph(t, bin)
	defer stop()
	ctx := context.Background()
	p := &Pipeline{E: e, RulesDir: filepath.Join(poc, "rules"), Validity: 600}
	var got []string
	for _, sec := range []int{60, 500, 600} {
		facts, _ := os.ReadFile(filepath.Join(ref, fmt.Sprintf("facts-t%d.nt", sec)))
		now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC).Add(time.Duration(sec) * time.Second)
		res, err := p.Run(ctx, facts, now)
		if err != nil {
			t.Fatal(err)
		}
		set := map[string]bool{}
		for _, l := range strings.Split(string(res.Verdicts), "\n") {
			s, pr, o, ok := splitNT(l)
			if !ok || pr != "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
				continue
			}
			d := strings.SplitN(strings.TrimPrefix(s, "https://w3id.org/sylvatwin/verdict/"), "/", 2)[0]
			set[fmt.Sprintf("%d\t%s\t%s", sec, d, strings.TrimPrefix(o, "https://w3id.org/sylvatwin/core#"))] = true
		}
		var rows []string
		for r := range set {
			rows = append(rows, r)
		}
		sort.Strings(rows)
		got = append(got, rows...)
		r, err := e.Select(ctx, `SELECT (COUNT(*) AS ?n) WHERE { GRAPH <`+GraphHistory+`> { ?d <https://w3id.org/sylvatwin/core#resolvedAt> ?r } }`)
		if err != nil {
			t.Fatal(err)
		}
		n := "?"
		if i := strings.Index(string(r), `"value":"`); i >= 0 {
			n = strings.SplitN(string(r)[i+9:], `"`, 2)[0]
		}
		got = append(got, fmt.Sprintf("%d\tresolues\t%s", sec, n))
	}
	want := strings.Split(strings.TrimSpace(string(mustReadFile(t, filepath.Join(ref, "verdict-sequence.tsv")))), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("verdicts differents\n--- rdflib\n%s\n--- oxigraph\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	t.Logf("sequence identique :\n%s", strings.Join(got, "\n"))
}

func mustReadFile(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
