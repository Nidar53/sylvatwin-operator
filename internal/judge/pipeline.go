package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Pipeline : ordre d'execution des regles (README de rules/) :
//
//	derives/0*.rq  (passe 1)  sur <facts>                 -> <drifts>
//	derives/10-*.rq (passe 2) sur <facts> + <drifts>      -> <drifts>
//	derives/11-*.rq (passe 3) sur <facts> + <drifts>      -> <drifts>
//	lifecycle/*.ru            met a jour <history>
//	verdict/*.rq              -> verdicts (renvoyes, non stockes)
//
// Les regles de derives/ sont ecrites sans clause GRAPH : on les evalue sur
// l'union des graphes voulus grace au parametre default-graph-uri du protocole
// SPARQL 1.1 (le graphe par defaut de la requete devient l'union de ces graphes
// nommes). AUCUNE COPIE : l'ABox (faits) n'est ecrite qu'une fois par cycle, dans
// son graphe nomme. Avant A4, le juge recopiait l'ABox dans le graphe par defaut
// a chaque passe (un PUT puis trois ADD), ce qui faisait croitre la base
// d'environ 4 Go par heure. Le graphe history n'est jamais inclus : les regles
// de derives/ ne doivent pas le voir.
type Pipeline struct {
	E        *Endpoint
	RulesDir string // .../sylvatwin-poc/rules
	// HistoryFile : copie durable du seul graphe <history> (A4bis). Oxigraph tourne
	// EN MEMOIRE (sans --location) : le PUT de l'ABox ne grossit plus aucun journal
	// d'ecriture. Le graphe history (age des derives) est exporte apres chaque
	// cycle (fichier temporaire puis rename) et recharge si le serveur l'a perdu
	// (redemarrage). Vide = pas de persistance.
	HistoryFile string
	// Validity : horizon de validite de l'information, en secondes (verdict).
	Validity float64
}

// Result : sorties d'un cycle de jugement.
type Result struct {
	Drifts   []byte // N-Triples des derives du cycle
	Verdicts []byte // N-Triples des verdicts
	Timings  map[string]time.Duration
}

// Run : un cycle complet. facts = N-Triples du greffier (+ TBox non requise :
// les regles n'utilisent que les types ecrits par le greffier).
func (p *Pipeline) Run(ctx context.Context, facts []byte, now time.Time) (*Result, error) {
	res := &Result{Timings: map[string]time.Duration{}}
	nowLit := now.UTC().Format("2006-01-02T15:04:05Z")
	seconds, err := os.ReadFile(filepath.Join(p.RulesDir, "verdict", "seconds.sparql"))
	if err != nil {
		return nil, err
	}
	prep := func(q string) string { return Prepare(q, nowLit, p.Validity, strings.TrimSpace(string(seconds))) }

	t0 := time.Now()
	if err := p.restoreHistory(ctx); err != nil {
		return nil, fmt.Errorf("rechargement de l'historique : %w", err)
	}
	if err := p.E.PutGraph(ctx, GraphFacts, facts); err != nil {
		return nil, fmt.Errorf("chargement des faits : %w", err)
	}
	res.Timings["load"] = time.Since(t0)

	// --- derives, en trois passes, sur le graphe par defaut
	files, err := filepath.Glob(filepath.Join(p.RulesDir, "derives", "*.rq"))
	if err != nil {
		return nil, err
	}
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
		// Graphes lus par cette passe : l'ABox ; plus les derives deja trouvees
		// aux passes precedentes (petit graphe, ecrit une fois par passe).
		in := []string{GraphFacts}
		if drifts.Len() > 0 {
			if err := p.E.PutGraph(ctx, GraphDrifts, drifts.Bytes()); err != nil {
				return nil, err
			}
			in = append(in, GraphDrifts)
		}
		for _, f := range group {
			q, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			out, err := p.E.ConstructIn(ctx, prep(string(q)), in)
			if err != nil {
				return nil, fmt.Errorf("%s : %w", filepath.Base(f), err)
			}
			drifts.Write(out)
		}
		res.Timings[fmt.Sprintf("passe%d", i+1)] = time.Since(t)
	}
	// <drifts> = derives du cycle (remplace celles du cycle precedent)
	if err := p.E.PutGraph(ctx, GraphDrifts, drifts.Bytes()); err != nil {
		return nil, err
	}
	res.Drifts = drifts.Bytes()

	// --- cycle de vie (SPARQL Update, graphes nommes)
	t := time.Now()
	lc, _ := filepath.Glob(filepath.Join(p.RulesDir, "lifecycle", "*.ru"))
	sort.Strings(lc)
	for _, f := range lc {
		u, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		if err := p.E.Update(ctx, prep(string(u))); err != nil {
			return nil, fmt.Errorf("%s : %w", filepath.Base(f), err)
		}
	}
	res.Timings["lifecycle"] = time.Since(t)
	if err := p.saveHistory(ctx); err != nil {
		return nil, fmt.Errorf("export de l'historique : %w", err)
	}

	// --- verdicts
	t = time.Now()
	vf, _ := filepath.Glob(filepath.Join(p.RulesDir, "verdict", "*.rq"))
	sort.Strings(vf)
	var verdicts bytes.Buffer
	for _, f := range vf {
		q, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out, err := p.E.Construct(ctx, prep(string(q)))
		if err != nil {
			return nil, fmt.Errorf("%s : %w", filepath.Base(f), err)
		}
		verdicts.Write(out)
	}
	res.Timings["verdict"] = time.Since(t)
	res.Verdicts = verdicts.Bytes()
	return res, nil
}

// restoreHistory : si le serveur n'a plus de graphe history (demarrage a vide)
// et que le fichier existe, le recharge AVANT tout jugement.
func (p *Pipeline) restoreHistory(ctx context.Context) error {
	if p.HistoryFile == "" {
		return nil
	}
	if has, err := p.historyNotEmpty(ctx); err != nil || has {
		return err
	}
	b, err := os.ReadFile(p.HistoryFile)
	if os.IsNotExist(err) || (err == nil && len(bytes.TrimSpace(b)) == 0) {
		return nil
	}
	if err != nil {
		return err
	}
	return p.E.PutGraph(ctx, GraphHistory, b)
}

func (p *Pipeline) historyNotEmpty(ctx context.Context) (bool, error) {
	out, err := p.E.Select(ctx, "ASK { GRAPH <"+GraphHistory+"> { ?s ?p ?o } }")
	if err != nil {
		return false, err
	}
	var r struct {
		Boolean bool `json:"boolean"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return false, err
	}
	return r.Boolean, nil
}

// saveHistory : exporte <history> (fichier temporaire, puis rename atomique).
func (p *Pipeline) saveHistory(ctx context.Context) error {
	if p.HistoryFile == "" {
		return nil
	}
	has, err := p.historyNotEmpty(ctx)
	if err != nil || !has {
		return err // rien a exporter (GET sur un graphe absent = 404)
	}
	b, err := p.E.GetGraph(ctx, GraphHistory)
	if err != nil {
		return err
	}
	tmp := p.HistoryFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.HistoryFile)
}

// FreshDeadline : echeance de fraicheur d'un cluster (rules/deadline).
type FreshDeadline struct {
	Cluster string
	At      time.Time     // maintenant + delai SPARQL
	Window  time.Duration // seuil de peremption (fenetre) du cluster
	Witness bool          // true : cluster a temoin (At se repousse a chaque Lease)
}

// Deadlines : les echeances rendues par rules/deadline. Les DELAIS et seuils
// viennent de SPARQL ; Go n'ajoute que des instants.
type Deadlines struct {
	Horizon time.Time // prochaine fin d'horizon de convergence (zero = aucune)
	Fresh   []FreshDeadline
}

// Min : la plus proche des echeances (zero = aucune).
func (d Deadlines) Min() time.Time {
	m := d.Horizon
	for _, f := range d.Fresh {
		if m.IsZero() || f.At.Before(m) {
			m = f.At
		}
	}
	return m
}

// NextDeadline : echeances utiles (rules/deadline/*.rq) : les DELAIS sont
// calcules par SPARQL ; Go ajoute seulement `now`.
func (p *Pipeline) NextDeadline(ctx context.Context, now time.Time) (Deadlines, error) {
	var out Deadlines
	q, err := os.ReadFile(filepath.Join(p.RulesDir, "deadline", "01-prochaine-echeance.rq"))
	if err != nil {
		return out, err
	}
	seconds, err := os.ReadFile(filepath.Join(p.RulesDir, "verdict", "seconds.sparql"))
	if err != nil {
		return out, err
	}
	nowLit := now.UTC().Format("2006-01-02T15:04:05Z")
	raw, err := p.E.Select(ctx, Prepare(string(q), nowLit, p.Validity, strings.TrimSpace(string(seconds))))
	if err != nil {
		return out, err
	}
	var r struct {
		Results struct {
			Bindings []map[string]struct {
				Value string `json:"value"`
			} `json:"bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return out, err
	}
	secs := func(b map[string]struct {
		Value string `json:"value"`
	}, k string) (time.Duration, bool) {
		v, ok := b[k]
		if !ok || v.Value == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(math.Ceil(f)) * time.Second, true
	}
	for _, b := range r.Results.Bindings {
		d, ok := secs(b, "delai")
		if !ok {
			continue
		}
		at := now.Add(d)
		switch b["genre"].Value {
		case "horizon":
			if out.Horizon.IsZero() || at.Before(out.Horizon) {
				out.Horizon = at
			}
		case "fraicheur":
			win, _ := secs(b, "fenetre")
			out.Fresh = append(out.Fresh, FreshDeadline{Cluster: b["cluster"].Value, At: at, Window: win,
				Witness: b["temoin"].Value == "true"})
		}
	}
	return out, nil
}

var secondsRe = regexp.MustCompile(`__SECONDS__\((\?\w+),\s*(\?\w+)\)`)

// Prepare : substitution des parametres d'une regle (meme logique que
// tests/verif_v1.py, pour que rdflib et Oxigraph executent le MEME texte).
func Prepare(q, now string, validity float64, seconds string) string {
	q = strings.ReplaceAll(q, "__NOW__", now)
	q = strings.ReplaceAll(q, "__VALIDITY__", fmt.Sprintf("%.1f", validity))
	return secondsRe.ReplaceAllStringFunc(q, func(m string) string {
		sm := secondsRe.FindStringSubmatch(m)
		return strings.ReplaceAll(strings.ReplaceAll(seconds, "$A", sm[1]), "$B", sm[2])
	})
}
