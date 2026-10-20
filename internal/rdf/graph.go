/*
Package rdf : ecriture Turtle minimale.

Pourquoi pas une bibliotheque RDF Go ? L'ecosysteme Go n'a pas d'equivalent
mature de rdflib. Comme l'operateur n'a besoin QUE d'ecrire (jamais de parser
ni de raisonner), 150 lignes suffisent et suppriment tout risque de dependance.
Le raisonnement reste en Python (detect.py + rdflib + SPARQL).
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

// Term : un terme RDF deja serialise en syntaxe Turtle.
type Term string

// Namespace du projet, identique a fact_writer.py v1.
const NS = "https://w3id.org/sylvatwin/k8s#"

// sanitize applique la convention v1 : tirets et points -> underscores.
// Indispensable : detect.py et la TBox dependent de ces noms exacts.
func sanitize(s string) string {
	r := strings.NewReplacer("-", "_", ".", "_", "/", "_", ":", "_")
	return r.Replace(s)
}

// IRI construit un individu du namespace projet.
// kind vide => pas de prefixe (convention des Deployments : :webapp).
//
// C'est la fonction d'identite deterministe : deux sources independantes
// (Git et Watch API) doivent converger sur la MEME URI.
func IRI(kind, name string) Term {
	if kind == "" {
		return Term(":" + sanitize(name))
	}
	return Term(":" + kind + "_" + sanitize(name))
}

// Class reference une classe de la TBox.
func Class(local string) Term { return Term(":" + local) }

// Prop reference une propriete de la TBox.
func Prop(local string) Term { return Term(":" + local) }

// Str : litteral chaine.
func Str(s string) Term {
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return Term(`"` + esc.Replace(s) + `"`)
}

// Int : litteral entier type. Le datatype est OBLIGATOIRE :
// sans lui, SPARQL comparerait la chaine "3" a l'entier 3 dans Q0.
func Int(i int) Term {
	return Term(`"` + strconv.Itoa(i) + `"^^xsd:integer`)
}

// DateTime : litteral xsd:dateTime.
func DateTime(t time.Time) Term {
	return Term(`"` + t.UTC().Format(time.RFC3339) + `"^^xsd:dateTime`)
}

// A est rdf:type, ecrit "a" en Turtle.
const A Term = "a"

type triple struct{ s, p, o Term }

// Graph : ensemble de triplets, sans doublon.
type Graph struct {
	set map[triple]struct{}
}

func New() *Graph { return &Graph{set: make(map[triple]struct{})} }

// Add insere un triplet. RDF est un ENSEMBLE : reinserer le meme triplet
// est sans effet, d'ou l'absence de set_single() ici (voir le controleur :
// le graphe est reconstruit entierement a chaque reconciliation).
func (g *Graph) Add(s, p, o Term) {
	if s == "" || p == "" || o == "" {
		return
	}
	g.set[triple{s, p, o}] = struct{}{}
}

func (g *Graph) Len() int { return len(g.set) }

// Serialize ecrit le graphe en Turtle, groupe par sujet et trie
// (sortie deterministe : deux reconciliations identiques donnent
// deux fichiers identiques, donc pas de churn Git ni de diff parasite).
func (g *Graph) Serialize(w io.Writer, tbox []byte) error {
	if _, err := fmt.Fprintf(w,
		"@prefix : <%s> .\n@prefix owl: <http://www.w3.org/2002/07/owl#> .\n"+
			"@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .\n"+
			"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n", NS); err != nil {
		return err
	}

	if len(tbox) > 0 {
		already := map[string]bool{":": true, "owl:": true, "rdfs:": true, "xsd:": true}
		pfx, body := splitTBox(tbox, already)
		if len(pfx) > 0 {
			if _, err := w.Write(append(pfx, '\n', '\n')); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte("### ---- TBox ----\n")); err != nil {
			return err
		}
		if _, err := w.Write(body); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n### ---- ABox (faits observes) ----\n")); err != nil {
			return err
		}
	}

	bySubject := map[Term]map[Term][]Term{}
	for t := range g.set {
		if bySubject[t.s] == nil {
			bySubject[t.s] = map[Term][]Term{}
		}
		bySubject[t.s][t.p] = append(bySubject[t.s][t.p], t.o)
	}

	subjects := make([]string, 0, len(bySubject))
	for s := range bySubject {
		subjects = append(subjects, string(s))
	}
	sort.Strings(subjects)

	for _, ss := range subjects {
		s := Term(ss)
		preds := make([]string, 0, len(bySubject[s]))
		for p := range bySubject[s] {
			preds = append(preds, string(p))
		}
		// "a" (rdf:type) toujours en premier, comme dans la sortie rdflib.
		sort.Slice(preds, func(i, j int) bool {
			if preds[i] == string(A) {
				return true
			}
			if preds[j] == string(A) {
				return false
			}
			return preds[i] < preds[j]
		})

		if _, err := fmt.Fprintf(w, "%s ", s); err != nil {
			return err
		}
		for i, ps := range preds {
			objs := bySubject[s][Term(ps)]
			strs := make([]string, 0, len(objs))
			for _, o := range objs {
				strs = append(strs, string(o))
			}
			sort.Strings(strs)

			sep := " ;\n    "
			if i == len(preds)-1 {
				sep = " .\n\n"
			}
			if _, err := fmt.Fprintf(w, "%s %s%s", ps, strings.Join(strs, ", "), sep); err != nil {
				return err
			}
		}
	}
	return nil
}

// splitTBox separe les declarations @prefix du corps de la TBox.
//
// Les prefixes de la TBox sont CONSERVES (et non supprimes) : sans cela, tout
// vocabulaire ajoute a l'ontologie (dcterms, skos, prov...) produirait un
// fichier syntaxiquement invalide. Seuls les prefixes deja ecrits en tete
// sont ignores, pour eviter les doublons.
func splitTBox(b []byte, already map[string]bool) (prefixes, body []byte) {
	var pfx, rest []string
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@prefix") || strings.HasPrefix(trimmed, "@base") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && already[fields[1]] {
				continue // deja declare en tete
			}
			pfx = append(pfx, trimmed)
			continue
		}
		rest = append(rest, line)
	}
	return []byte(strings.Join(pfx, "\n")), []byte(strings.Join(rest, "\n"))
}
