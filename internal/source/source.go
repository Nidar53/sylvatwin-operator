/*
Package source : les SOURCES DE FAITS par cluster (decision 1 : interface qui
garde H1 hub-dans-le-management et H2 hub+agents possibles).

	FactSource (interface)
	  |- Live    : lecture directe d'une API Kubernetes (cache controller-runtime,
	  |            Secrets en metadonnees seulement) -> H0 (hors cluster) et H1
	  |- (futur) Agent : un agent dans le workload pousse ses Snapshots -> H2

Le reste de l'operateur (mapper, juge) ne voit que des mapper.Snapshot : il ne
sait pas si les faits viennent d'un cache local, d'une API distante ou d'un agent.

SECURITE (decisions 2 et 4) :
  - le kubeconfig admin fourni par CAPI n'est lu qu'en MEMOIRE pour construire un
    rest.Config ; il n'est jamais ecrit (graphe, fichier, log). Toutes les
    requetes vers le workload sont faites EN IMPERSONATION du compte en lecture
    seule sylvatwin-reader : l'identite admin authentifie, les droits effectifs
    sont ceux de sylvatwin-reader ;
  - les Secrets sont lus en PartialObjectMetadata (metadonnees seulement).
*/
package source

import (
	"context"

	"github.com/orange/sylvatwin-operator/internal/mapper"
)

// FactSource : produit l'instantane courant d'UN cluster.
type FactSource interface {
	// Name : nom logique du cluster (= nom du Cluster CAPI qui le decrit).
	Name() string
	// Snapshot : objets observes + statut + portee. Si le cluster est
	// injoignable, renvoie le DERNIER instantane reussi avec Status
	// "unreachable" (P3 : on ne retire pas les faits en silence).
	Snapshot(ctx context.Context) *mapper.Snapshot
	// Close : libere les connexions (watches).
	Close()
}

// Spec : ce qu'on observe dans un cluster (portee P3). Chaque entree devient une
// mapper.Scope : seules ces paires (Kind, namespace) sont declarees observees.
type Spec struct {
	Kind      string // Kind Kubernetes ET nom de classe de l'ontologie
	Group     string
	Version   string
	Resource  string // pluriel REST
	Namespace string // "" ou "*" = tous
	// MetadataOnly : PartialObjectMetadata (obligatoire pour Secret).
	MetadataOnly bool
	// NoScope : objet lu pour le mapper, mais pas declare comme portee (ex.
	// ConfigMaps des NF : on ne raisonne jamais sur leur absence).
	NoScope bool
}

// WorkloadSpecs : portee observee dans un cluster workload (namespace des NF).
func WorkloadSpecs(nfNamespace string) []Spec {
	return []Spec{
		{Kind: "Node", Version: "v1", Resource: "nodes"},
		{Kind: "Namespace", Version: "v1", Resource: "namespaces"},
		{Kind: "Pod", Version: "v1", Resource: "pods", Namespace: nfNamespace},
		{Kind: "ReplicaSet", Group: "apps", Version: "v1", Resource: "replicasets", Namespace: nfNamespace},
		{Kind: "Deployment", Group: "apps", Version: "v1", Resource: "deployments", Namespace: nfNamespace},
		{Kind: "Secret", Version: "v1", Resource: "secrets", Namespace: nfNamespace, MetadataOnly: true},
		{Kind: "ConfigMap", Version: "v1", Resource: "configmaps", Namespace: nfNamespace, NoScope: true},
	}
}

// ManagementSpecs : portee observee dans le cluster de management.
func ManagementSpecs() []Spec {
	return []Spec{
		{Kind: "Namespace", Version: "v1", Resource: "namespaces"},
		{Kind: "Cluster", Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"},
		{Kind: "Machine", Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "machines"},
		{Kind: "GitRepository", Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		{Kind: "Kustomization", Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Kind: "HelmRelease", Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		{Kind: "HelmChart", Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmcharts", NoScope: true},
	}
}
