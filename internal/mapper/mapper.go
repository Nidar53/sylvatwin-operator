package mapper

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// Input : tout ce qu'il faut pour construire le graphe des faits d'un cycle.
type Input struct {
	Snapshots []*Snapshot
	Desired   []*Desired
	Flux      FluxContext
}

// Build : graphe des FAITS (observe + desire), toutes couches, tous clusters.
// Fonction pure : memes entrees => meme graphe (testable sur une photo).
func Build(in Input) *rdf.Graph {
	g := rdf.New()
	endpoints := map[string]string{}
	for _, s := range in.Snapshots {
		if s.Endpoint != "" {
			endpoints[s.Endpoint] = s.Cluster
		}
	}
	for _, s := range in.Snapshots {
		mapCluster(g, s)
		mapL2(g, s)
		mapL1(g, s, endpoints)
		mapL3(g, s, in.Flux)
		mapL4(g, s)
	}
	for _, d := range in.Desired {
		mapDesired(g, d)
	}
	return g
}

// mapDesired : cote DESIRE (Git a la revision exacte), sur les MEMES IRI que
// l'observe (Option C). Desire "inconnu" (sha different de l'artefact Flux) :
// on n'ecrit RIEN (pas de fausse derive fondee sur un desire douteux).
func mapDesired(g *rdf.Graph, d *Desired) {
	if d == nil || !d.Known {
		return
	}
	for _, o := range d.Objects {
		if o.GetKind() != "Deployment" {
			continue
		}
		dep := rdf.IRI(iri.Resource(d.TargetCluster, o.GetNamespace(), "Deployment", "apps", o.GetName()))
		g.Add(dep, rdf.Type, rdf.C("Deployment"))
		rep, ok := i64(o.Object, "spec", "replicas")
		if !ok {
			rep = 1
		}
		g.Add(dep, rdf.C("desiredReplicas"), rdf.Int(rep))
		if d.Revision != "" {
			g.Add(dep, rdf.C("desiredRevision"), rdf.Str(d.Revision))
		}
		sel, _, _ := unstructured.NestedStringMap(o.Object, "spec", "template", "spec", "nodeSelector")
		for k, v := range sel {
			li := rdf.IRI(iri.NodeSelectorOf(dep.Value(), k))
			g.Add(dep, rdf.C("requiresNodeLabel"), li)
			g.Add(li, rdf.Type, rdf.C("Label"))
			g.Add(li, rdf.C("hasKey"), rdf.Str(k))
			g.Add(li, rdf.C("hasValue"), rdf.Str(v))
		}
	}
}
