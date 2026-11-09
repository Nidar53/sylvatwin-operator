package rdf

import (
	"strings"
	"testing"
	"time"
)

func TestNTriplesEtTurtle(t *testing.T) {
	g := New()
	s := IRI("https://w3id.org/sylvatwin/res/c/_/Node/n1")
	g.Add(s, Type, C("Node"))
	g.Add(s, C("resourceName"), Str(`a "b"`+"\n"))
	g.Add(s, C("desiredReplicas"), Int(3))
	g.Add(s, C("reconcileInterval"), Dec(120))
	g.Add(s, C("suspended"), Bool(false))
	g.Add(s, C("observedAt"), DateTime(time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)))
	g.Add(s, C("resourceName"), Str(`a "b"`+"\n")) // doublon : ignore
	g.Add(IRI(""), Type, C("Node"))                 // sujet vide : ignore
	if g.Len() != 6 {
		t.Fatalf("Len = %d, attendu 6", g.Len())
	}
	var nt strings.Builder
	if err := g.WriteNTriples(&nt); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<https://w3id.org/sylvatwin/res/c/_/Node/n1> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://w3id.org/sylvatwin/core#Node> .`,
		`"a \"b\"\n"`,
		`"3"^^<http://www.w3.org/2001/XMLSchema#integer>`,
		`"120.0"^^<http://www.w3.org/2001/XMLSchema#decimal>`,
		`"false"^^<http://www.w3.org/2001/XMLSchema#boolean>`,
		`"2026-11-09T12:00:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>`,
	} {
		if !strings.Contains(nt.String(), want) {
			t.Errorf("N-Triples sans %s :\n%s", want, nt.String())
		}
	}
	// la premiere ligne doit etre rdf:type (tri deterministe)
	if !strings.Contains(strings.Split(nt.String(), "\n")[0], "#type") {
		t.Errorf("rdf:type pas en tete")
	}
	var ttl strings.Builder
	if err := g.WriteTurtle(&ttl, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ttl.String(), "a :Node") || !strings.Contains(ttl.String(), ":desiredReplicas \"3\"^^xsd:integer") {
		t.Errorf("Turtle inattendu :\n%s", ttl.String())
	}
}
