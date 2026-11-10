/*
Package mapper : faits observes -> triplets de l'ontologie SylvaTwin v1.

C'est le GREFFIER. Il ne compare rien, ne juge rien : il recopie des champs
Kubernetes en triplets. Toute la detection est dans rules/ (sylvatwin-poc),
executee par le juge (internal/judge). Aucune classe Drift n'est ecrite ici.

Entree : des Snapshots (un par cluster), produits par internal/source.
Les objets sont "non structures" (map JSON) : le mapper lit les CRD Flux et CAPI
sans dependre de leurs paquets Go (pas de montee de version, et le meme code lit
une photo JSON enregistree pour les tests).
*/
package mapper

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Status d'observation d'un cluster (valeurs de :observationStatus).
const (
	StatusOK          = "ok"
	StatusUnreachable = "unreachable"
	StatusForbidden   = "forbidden"
)

// Scope : un type observe dans un namespace ("*" = tous).
type Scope struct {
	Kind      string // nom local de la classe de l'ontologie : "Secret", "Node"...
	Namespace string
}

// Snapshot : ce que le greffier a vu d'UN cluster au dernier cycle.
type Snapshot struct {
	// Cluster : nom logique (= nom du Cluster CAPI qui le decrit, si connu).
	Cluster string
	// Endpoint : "host:port" de l'API observee (sert a clusterDescribedBy et a
	// targetsCluster ; jamais d'identifiant de connexion).
	Endpoint string
	// Status : ok, unreachable, forbidden.
	Status string
	// LastSync : derniere observation complete REUSSIE (peut etre ancienne si
	// Status != ok : les objets sont alors ceux de ce dernier instantane).
	LastSync time.Time
	// ObservedAt : horodatage ecrit sur chaque objet (= LastSync en pratique).
	ObservedAt time.Time
	// Scopes : portee effectivement observee (P3).
	Scopes []Scope
	// Objects : objets observes. Les Secrets sont des PartialObjectMetadata
	// (metadonnees seulement) : le mapper n'accede jamais a .data.
	Objects []*unstructured.Unstructured

	// --- Watch temoin et fraicheur (partie 2) ---------------------------------
	// WitnessLeaseName : nom de la Lease de noeud surveillee comme temoin de vie
	// du canal (ex. "<node>" dans kube-node-lease). Vide = pas de temoin configure.
	WitnessLeaseName string
	// LastWitnessRenew : u_c(t), dernier renouvellement OBSERVE PAR WATCH de
	// cette Lease (independant de LastSync : c'est la preuve de vie du canal,
	// pas la derniere reconstruction du graphe).
	LastWitnessRenew time.Time
	// FreshnessHorizon : H_frais (theta), secondes.
	FreshnessHorizon float64
}

// Desired : le cote desire (Git a la revision exacte) pour un cluster cible.
type Desired struct {
	// Revision : "branche@sha1:<sha>" (= artefact Flux), ou "" si inconnu.
	Revision string
	// Known : false si le sha clone differe du sha de l'artefact Flux (desire
	// declare "inconnu" : aucun fait desired* n'est ecrit).
	Known bool
	// TargetCluster : cluster ou ces objets doivent exister.
	TargetCluster string
	// Objects : manifestes rendus (kustomize build), namespace resolu.
	Objects []*unstructured.Unstructured
}
