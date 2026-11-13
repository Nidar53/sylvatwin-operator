package source

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/mapper"
)

// FluxContext : informations dont le mapper Flux a besoin, SANS LIRE AUCUN
// SECRET (regle 4 : seul le kubeconfig d'un cluster CAPI peut etre lu, en
// memoire, pour se connecter).
//
// KubeconfigTargets ("ns/secret" -> cluster cible d'une Kustomization distante) :
//  1. convention Cluster API : un Secret "<cluster>-kubeconfig" dans le namespace
//     d'un Cluster CAPI decouvert designe ce cluster (deduit du NOM, contenu non lu) ;
//  2. sinon, correspondance DECLAREE par l'utilisateur (--kubeconfig-target
//     ns/secret=cluster), car le contenu du Secret ne doit pas etre lu.
//     Sans correspondance : targetsCluster n'est pas ecrit (on n'invente rien).
//
// HelmChartRevisions : status.artifact.revision des HelmChart (objets non secrets).
func FluxContext(s *mapper.Snapshot, discovered []Discovered, declared map[string]string) mapper.FluxContext {
	fc := mapper.FluxContext{KubeconfigTargets: map[string]string{}, HelmChartRevisions: map[string]string{}}
	for _, d := range discovered {
		// Nom CANONIQUE (namespace/nom), meme forme que le provider CAPI de
		// multicluster-runtime (iri.CanonicalCluster) : le nom nu du Cluster CAPI
		// ne correspond au nom du FactSource engage que par coincidence.
		fc.KubeconfigTargets[d.Namespace+"/"+d.Name+"-kubeconfig"] = iri.CanonicalCluster(d.Namespace, d.Name)
	}
	for k, v := range declared {
		fc.KubeconfigTargets[k] = v
	}
	if s == nil {
		return fc
	}
	for _, o := range s.Objects {
		if o.GetKind() == "HelmChart" {
			if rev, _, _ := unstructured.NestedString(o.Object, "status", "artifact", "revision"); rev != "" {
				fc.HelmChartRevisions[o.GetNamespace()+"/"+o.GetName()] = rev
			}
		}
	}
	return fc
}

// ParseTargets : "ns/secret=cluster,ns2/s2=c2" -> map.
func ParseTargets(s string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		p := strings.SplitN(strings.TrimSpace(kv), "=", 2)
		if len(p) == 2 && p[0] != "" && p[1] != "" {
			out[p[0]] = p[1]
		}
	}
	return out
}
