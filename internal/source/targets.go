package source

import (
	"fmt"
	"strings"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/mapper"
)

// ResolveTargets : resout les cibles DECLAREES (--kubeconfig-target, D2) vers
// le nom CANONIQUE du cluster workload (iri.CanonicalCluster), en UN SEUL
// endroit (avec iri.CanonicalCluster lui-meme).
//
//   - une valeur DEJA canonique ("namespace/nom", detectee par la presence de
//     "/" : un nom Kubernetes DNS-1123 n'en contient jamais) est gardee telle
//     quelle, SANS verification contre le management (compatibilite ascendante
//     et cas d'un cluster pas encore decouvert au moment du cycle) ;
//   - une valeur NUE (pas de "/") est cherchee parmi les Cluster CAPI observes
//     dans le snapshot du management (mg). Zero ou plusieurs correspondances :
//     ERREUR EXPLICITE (on n'invente rien, meme logique que twin.Twin.desired,
//     "cluster cible inconnu") ; la cible est alors OMISE de la carte renvoyee
//     (le desire restera "inconnu" pour toute Kustomization qui la visait).
//
// Fonction pure sur un instantane (mg peut etre nil : toute cible nue devient
// alors une erreur, aucune donnee pour la resoudre).
func ResolveTargets(mg *mapper.Snapshot, declared map[string]string) (resolved map[string]string, errs []string) {
	resolved = make(map[string]string, len(declared))
	for k, v := range declared {
		if v == "" {
			continue
		}
		if strings.Contains(v, "/") {
			resolved[k] = v
			continue
		}
		matches := findCAPIClustersByName(mg, v)
		switch len(matches) {
		case 1:
			resolved[k] = iri.CanonicalCluster(matches[0].Namespace, matches[0].Name)
		case 0:
			errs = append(errs, fmt.Sprintf("cible %s=%s : aucun Cluster CAPI nomme %q dans le management (nom nu non resolu)", k, v, v))
		default:
			var ns []string
			for _, m := range matches {
				ns = append(ns, m.Namespace)
			}
			errs = append(errs, fmt.Sprintf("cible %s=%s : nom %q ambigu, plusieurs Cluster CAPI le portent (namespaces %s)", k, v, v, strings.Join(ns, ", ")))
		}
	}
	return resolved, errs
}

// findCAPIClustersByName : parmi les Cluster CAPI du management (meme lecture
// que DiscoveredFromSnapshot), ceux dont le NOM (pas le namespace) correspond.
func findCAPIClustersByName(mg *mapper.Snapshot, name string) []Discovered {
	var out []Discovered
	for _, d := range DiscoveredFromSnapshot(mg) {
		if d.Name == name {
			out = append(out, d)
		}
	}
	return out
}
