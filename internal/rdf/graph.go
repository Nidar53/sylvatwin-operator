/*
Package rdf : ecriture RDF minimale pour l'ontologie SylvaTwin v1 (espace core#).

Pourquoi pas une bibliotheque RDF Go ? L'operateur n'a besoin QUE d'ecrire des
triplets (jamais de parser ni de raisonner) : le juge (moteur SPARQL externe)
fait le reste. Ce paquet produit du N-Triples (une ligne par triplet, format
canonique, sans prefixe : ideal pour le Graph Store Protocol) et du Turtle
lisible.

Invariant greffier/juge : ce paquet ne connait QUE des termes. Aucune logique de
detection ici, ni nulle part dans l'operateur.
*/
package rdf

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Espaces de noms de la v1.
const (
	Core   = "https://w3id.org/sylvatwin/core#"
	XSD    = "http://www.w3.org/2001/XMLSchema#"
	RDFNS  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFSNS = "http://www.w3.org/2000/01/rdf-schema#"
)

// Kind de terme.
type kind uint8

const (
	kIRI kind = iota
	kLiteral
)

// Term : un terme RDF (IRI ou litteral type). Comparable : sert de cle de map.
type Term struct {
	k     kind
	value string // IRI complete, ou forme lexicale du litteral
	dt    string // datatype (IRI complete) pour un litteral ; "" = xsd:string
}

// IRI construit un terme IRI a partir d'une IRI COMPLETE.
func IRI(full string) Term { return Term{k: kIRI, value: full} }

// C : un terme de l'ontologie (classe ou propriete) dans l'espace core#.
func C(local string) Term { return IRI(Core + local) }

// Type : rdf:type.
var Type = IRI(RDFNS + "type")

// Label : rdfs:label.
var Label = IRI(RDFSNS + "label")

// Str : litteral chaine (xsd:string, forme simple en Turtle/N-Triples).
func Str(s string) Term { return Term{k: kLiteral, value: s} }

// Int : litteral xsd:integer. Le datatype est OBLIGATOIRE : sans lui, SPARQL
// comparerait la chaine "3" a l'entier 3.
func Int(i int64) Term {
	return Term{k: kLiteral, value: strconv.FormatInt(i, 10), dt: XSD + "integer"}
}

// Dec : litteral xsd:decimal (secondes des parametres Flux, par ex.).
func Dec(f float64) Term {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return Term{k: kLiteral, value: s, dt: XSD + "decimal"}
}

// Bool : litteral xsd:boolean.
func Bool(b bool) Term { return Term{k: kLiteral, value: strconv.FormatBool(b), dt: XSD + "boolean"} }

// AnyURI : litteral xsd:anyURI (URL Git).
func AnyURI(s string) Term { return Term{k: kLiteral, value: s, dt: XSD + "anyURI"} }

// DateTime : litteral xsd:dateTime en UTC, precision a la seconde.
func DateTime(t time.Time) Term {
	return Term{k: kLiteral, value: t.UTC().Format("2006-01-02T15:04:05Z"), dt: XSD + "dateTime"}
}

// IsIRI indique si le terme est une IRI.
func (t Term) IsIRI() bool { return t.k == kIRI }

// Value renvoie l'IRI complete ou la forme lexicale.
func (t Term) Value() string { return t.value }

// Datatype renvoie le datatype ("" pour xsd:string ou une IRI).
func (t Term) Datatype() string { return t.dt }

// NT serialise le terme en N-Triples.
func (t Term) NT() string {
	if t.k == kIRI {
		return "<" + escapeIRI(t.value) + ">"
	}
	lex := `"` + escapeLiteral(t.value) + `"`
	if t.dt == "" {
		return lex
	}
	return lex + "^^<" + t.dt + ">"
}

// escapeLiteral : echappements N-Triples / Turtle d'un litteral court.
func escapeLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeIRI : une IRI ne peut contenir ni espace, ni <>"{}|^`\ ; les IRI SylvaTwin
// sont deja encodees par le paquet iri, ceci n'est qu'un garde-fou.
func escapeIRI(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r <= 0x20 || strings.ContainsRune("<>\"{}|^`\\", r) {
			fmt.Fprintf(&b, "%%%02X", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type triple struct{ s, p, o Term }

// Graph : ensemble de triplets, sans doublon (RDF est un ensemble).
type Graph struct {
	set map[triple]struct{}
}

// New cree un graphe vide.
func New() *Graph { return &Graph{set: make(map[triple]struct{})} }

// Add insere un triplet. Sujet et predicat doivent etre des IRI ; un terme vide
// est ignore (champ Kubernetes absent = pas de fait).
func (g *Graph) Add(s, p, o Term) {
	if s.value == "" || p.value == "" || !s.IsIRI() || !p.IsIRI() {
		return
	}
	if o.k == kIRI && o.value == "" {
		return
	}
	g.set[triple{s, p, o}] = struct{}{}
}

// Len : nombre de triplets.
func (g *Graph) Len() int { return len(g.set) }

// Has : test d'appartenance (tests unitaires).
func (g *Graph) Has(s, p, o Term) bool { _, ok := g.set[triple{s, p, o}]; return ok }

// Objects : objets de (s, p) (tests unitaires et status).
func (g *Graph) Objects(s, p Term) []Term {
	var out []Term
	for t := range g.set {
		if t.s == s && t.p == p {
			out = append(out, t.o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NT() < out[j].NT() })
	return out
}
func (g *Graph) sorted() []triple {
	ts := make([]triple, 0, len(g.set))
	for t := range g.set {
		ts = append(ts, t)
	}
	sort.Slice(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if a.s.value != b.s.value {
			return a.s.value < b.s.value
		}
		if a.p != b.p {
			if a.p == Type {
				return true
			}
			if b.p == Type {
				return false
			}
			return a.p.value < b.p.value
		}
		return a.o.NT() < b.o.NT()
	})
	return ts
}

// WriteNTriples : sortie deterministe (triee), une ligne par triplet.
func (g *Graph) WriteNTriples(w io.Writer) error {
	for _, t := range g.sorted() {
		if _, err := fmt.Fprintf(w, "%s %s %s .\n", t.s.NT(), t.p.NT(), t.o.NT()); err != nil {
			return err
		}
	}
	return nil
}

// WriteTurtle : Turtle lisible (groupe par sujet, prefixes core/xsd), deterministe.
func (g *Graph) WriteTurtle(w io.Writer, header string) error {
	if header != "" {
		for _, l := range strings.Split(strings.TrimRight(header, "\n"), "\n") {
			if _, err := fmt.Fprintf(w, "# %s\n", l); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "@prefix : <%s> .\n@prefix xsd: <%s> .\n@prefix rdfs: <%s> .\n\n", Core, XSD, RDFSNS); err != nil {
		return err
	}
	short := func(t Term) string {
		if t.k == kIRI {
			if t == Type {
				return "a"
			}
			if strings.HasPrefix(t.value, Core) && isPN(t.value[len(Core):]) {
				return ":" + t.value[len(Core):]
			}
			if strings.HasPrefix(t.value, RDFSNS) && isPN(t.value[len(RDFSNS):]) {
				return "rdfs:" + t.value[len(RDFSNS):]
			}
			return t.NT()
		}
		lex := `"` + escapeLiteral(t.value) + `"`
		if t.dt == "" {
			return lex
		}
		if strings.HasPrefix(t.dt, XSD) {
			return lex + "^^xsd:" + t.dt[len(XSD):]
		}
		return lex + "^^<" + t.dt + ">"
	}
	ts := g.sorted()
	for i := 0; i < len(ts); {
		s := ts[i].s
		if _, err := fmt.Fprintf(w, "%s", short(s)); err != nil {
			return err
		}
		first := true
		for i < len(ts) && ts[i].s == s {
			p := ts[i].p
			var objs []string
			for i < len(ts) && ts[i].s == s && ts[i].p == p {
				objs = append(objs, short(ts[i].o))
				i++
			}
			sep := " ;\n    "
			if first {
				sep = "\n    "
				first = false
			}
			if _, err := fmt.Fprintf(w, "%s%s %s", sep, short(p), strings.Join(objs, " , ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, " .\n\n"); err != nil {
			return err
		}
	}
	return nil
}

// isPN : nom local utilisable en notation prefixee sans echappement.
func isPN(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && (r >= '0' && r <= '9'))
		if !ok {
			return false
		}
	}
	return true
}
