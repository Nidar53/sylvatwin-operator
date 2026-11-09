package mapper

import (
	"strings"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

const (
	gSource    = "source.toolkit.fluxcd.io"
	gKustomize = "kustomize.toolkit.fluxcd.io"
	gHelm      = "helm.toolkit.fluxcd.io"
)

// FluxContext : informations que le mapper Flux ne peut pas lire lui-meme.
type FluxContext struct {
	// KubeconfigTargets : "ns/secret" -> nom du cluster reel cible par ce
	// kubeconfig. Rempli par la source (lecture du champ server en MEMOIRE,
	// jamais ecrit) : le mapper ne voit jamais le contenu d'un Secret.
	KubeconfigTargets map[string]string
	// HelmChartRevisions : "ns/name" du HelmChart -> status.artifact.revision.
	HelmChartRevisions map[string]string
}

// mapL3 : GitRepository, Kustomization, HelmRelease (management).
func mapL3(g *rdf.Graph, s *Snapshot, fc FluxContext) {
	at := s.ObservedAt
	artifacts := map[string]string{} // "ns/name" du GitRepository -> revision
	for _, o := range s.Objects {
		if groupOf(o) == gSource && kindOf(o) == "GitRepository" {
			artifacts[o.GetNamespace()+"/"+o.GetName()] = str(o.Object, "status", "artifact", "revision")
		}
	}
	for _, o := range s.Objects {
		switch {
		case groupOf(o) == gSource && kindOf(o) == "GitRepository":
			r := identity(g, s.Cluster, o, at)
			g.Add(r, rdf.Type, rdf.C("GitRepository"))
			g.Add(r, rdf.C("apiGroup"), rdf.Str(gSource))
			if u := str(o.Object, "spec", "url"); u != "" {
				g.Add(r, rdf.C("gitUrl"), rdf.AnyURI(u))
			}
			if b := str(o.Object, "spec", "ref", "branch"); b != "" {
				g.Add(r, rdf.C("gitBranch"), rdf.Str(b))
			}
			fluxCommon(g, r, o.Object)
		case groupOf(o) == gKustomize && kindOf(o) == "Kustomization":
			k := identity(g, s.Cluster, o, at)
			g.Add(k, rdf.Type, rdf.C("Kustomization"))
			g.Add(k, rdf.C("apiGroup"), rdf.Str(gKustomize))
			fluxCommon(g, k, o.Object)
			if sk := str(o.Object, "spec", "sourceRef", "kind"); sk == "GitRepository" {
				sns := str(o.Object, "spec", "sourceRef", "namespace")
				if sns == "" {
					sns = o.GetNamespace()
				}
				sn := str(o.Object, "spec", "sourceRef", "name")
				g.Add(k, rdf.C("sourcedFrom"), rdf.IRI(iri.Resource(s.Cluster, sns, "GitRepository", gSource, sn)))
				if rev := artifacts[sns+"/"+sn]; rev != "" {
					g.Add(k, rdf.C("desiredRevision"), rdf.Str(rev))
				}
			}
			if rev := str(o.Object, "status", "lastAppliedRevision"); rev != "" {
				g.Add(k, rdf.C("actualRevision"), rdf.Str(rev))
			}
			target(g, s.Cluster, k, o.GetNamespace(), o.Object, fc)
			// Inventaire -> managedBy (pont L3 -> L2), dans le cluster cible.
			tgt := targetCluster(s.Cluster, o.GetNamespace(), o.Object, fc)
			if tgt != "" {
				inv, _, _ := unstructuredSlice(o.Object, "status", "inventory", "entries")
				for _, e := range inv {
					id, _ := e["id"].(string)
					if ri, ok := inventoryIRI(tgt, id); ok {
						g.Add(rdf.IRI(ri), rdf.C("managedBy"), k)
					}
				}
			}
		case groupOf(o) == gHelm && kindOf(o) == "HelmRelease":
			h := identity(g, s.Cluster, o, at)
			g.Add(h, rdf.Type, rdf.C("HelmRelease"))
			g.Add(h, rdf.C("apiGroup"), rdf.Str(gHelm))
			fluxCommon(g, h, o.Object)
			if m := str(o.Object, "spec", "driftDetection", "mode"); m != "" {
				g.Add(h, rdf.C("driftDetectionMode"), rdf.Str(m))
			} else {
				g.Add(h, rdf.C("driftDetectionMode"), rdf.Str("disabled"))
			}
			// desire : artefact du HelmChart genere (<ns>-<nom>) ; observe : historique
			if rev := fc.HelmChartRevisions[o.GetNamespace()+"/"+o.GetNamespace()+"-"+o.GetName()]; rev != "" {
				g.Add(h, rdf.C("desiredRevision"), rdf.Str(rev))
			}
			hist, _, _ := unstructuredSlice(o.Object, "status", "history")
			if len(hist) > 0 {
				if v, _ := hist[0]["chartVersion"].(string); v != "" {
					g.Add(h, rdf.C("actualRevision"), rdf.Str(v))
				}
			}
			target(g, s.Cluster, h, o.GetNamespace(), o.Object, fc)
		}
	}
}

// fluxCommon : suspended (false si absent), ready, intervalles en secondes.
func fluxCommon(g *rdf.Graph, s rdf.Term, o map[string]any) {
	susp, _ := boolean(o, "spec", "suspend")
	g.Add(s, rdf.C("suspended"), rdf.Bool(susp))
	if st := condition(o, "Ready"); st != "" {
		g.Add(s, rdf.C("ready"), rdf.Bool(st == "True"))
	}
	if v, ok := seconds(str(o, "spec", "interval")); ok {
		g.Add(s, rdf.C("reconcileInterval"), rdf.Dec(v))
	}
	if v, ok := seconds(str(o, "spec", "timeout")); ok {
		g.Add(s, rdf.C("reconcileTimeout"), rdf.Dec(v))
	}
	if v, ok := seconds(str(o, "spec", "retryInterval")); ok {
		g.Add(s, rdf.C("retryInterval"), rdf.Dec(v))
	}
}

func targetCluster(local, ns string, o map[string]any, fc FluxContext) string {
	name := str(o, "spec", "kubeConfig", "secretRef", "name")
	if name == "" {
		return local
	}
	return fc.KubeconfigTargets[ns+"/"+name] // "" si inconnu : on n'invente rien
}

func target(g *rdf.Graph, local string, s rdf.Term, ns string, o map[string]any, fc FluxContext) {
	if t := targetCluster(local, ns, o, fc); t != "" {
		g.Add(s, rdf.C("targetsCluster"), clusterIRI(t))
	}
}

// managedByLabels : objets produits par une HelmRelease ou une Kustomization
// (labels poses par Flux), quand aucun inventaire n'est disponible.
func managedByLabels(g *rdf.Graph, cluster string, s rdf.Term, objNS string, l map[string]string) {
	if n := l["helm.toolkit.fluxcd.io/name"]; n != "" {
		ns := l["helm.toolkit.fluxcd.io/namespace"]
		if ns == "" {
			ns = objNS
		}
		g.Add(s, rdf.C("managedBy"), rdf.IRI(iri.Resource(cluster, ns, "HelmRelease", gHelm, n)))
	}
}

// inventoryIRI : entree d'inventaire Flux "<ns>_<nom>_<groupe>_<Kind>" -> IRI.
// Les noms Kubernetes ne contiennent pas '_', le decoupage est donc sans
// ambiguite (format documente par Flux : ObjMetadata).
func inventoryIRI(cluster, id string) (string, bool) {
	parts := strings.Split(id, "_")
	if len(parts) != 4 {
		return "", false
	}
	ns, name, group, kind := parts[0], parts[1], parts[2], parts[3]
	return iri.Resource(cluster, ns, kind, group, name), true
}

func unstructuredSlice(o map[string]any, path ...string) ([]map[string]any, bool, error) {
	cur := any(o)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		cur = m[p]
	}
	sl, ok := cur.([]any)
	if !ok {
		return nil, false, nil
	}
	out := make([]map[string]any, 0, len(sl))
	for _, e := range sl {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, true, nil
}
