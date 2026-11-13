/*
Package iri : regles d'identite des individus SylvaTwin v1, en UN seul endroit.

Decisions (ontologie sylvatwin-core-v1.ttl, D1 et D1-bis) :

	ressource native  : https://w3id.org/sylvatwin/res/<cluster>/<namespace>/<Kind>/<nom>
	CRD               : https://w3id.org/sylvatwin/res/<cluster>/<namespace>/<Kind>.<groupe>/<nom>
	sans namespace    : segment "_" a la place du namespace
	cluster reel      : https://w3id.org/sylvatwin/res/<cluster>
	label observe     : <IRI de l'objet>/label/<cle encodee>
	exigence Git      : <IRI du Deployment>/nodeSelector/<cle encodee>
	fonction reseau   : .../res/<cluster>/<ns>/NetworkFunction/<nf>
	interface de NF   : <IRI de la NF>/interface/<nom>
	PLMN, tranche     : .../res/_/PLMN/<mcc>-<mnc>, .../res/_/Slice/sst-<n>[-sd-<sd>]
	portee observee   : .../res/<cluster>/_/scope/<Kind>/<ns ou _all>

Les noms Kubernetes (DNS-1123) ne contiennent que [a-z0-9.-] : on ne les modifie
PAS (contrairement a la v0 qui remplacait '-' par '_'). Seules les cles de labels
peuvent contenir '/', encode en %2F, comme dans l'exemple de reference.

L'uid n'entre PAS dans l'IRI : Git ne le connait pas, et l'IRI doit etre la meme
cote desire et cote observe (Option C).
*/
package iri

import (
	"net/url"
	"strings"
)

// Base des individus.
const Base = "https://w3id.org/sylvatwin/res/"

// NoNS : segment des objets sans namespace.
const NoNS = "_"

// segment encode un segment de chemin (noms DNS-1123 : inchanges ; le reste est
// percent-encode, '/' compris).
func segment(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "/", "%2F")
}

// Cluster : IRI du cluster reel (KubernetesCluster).
func Cluster(cluster string) string { return Base + segment(cluster) }

// CanonicalCluster : nom CANONIQUE d'un cluster workload decrit par un Cluster
// CAPI (namespace, nom), SOUS LA MEME FORME que multicluster-runtime/providers/
// cluster-api (types.NamespacedName.String(), "namespace/nom", cf. provider.go
// v0.24.1 : multicluster.ClusterName(req.NamespacedName.String())). UN SEUL
// endroit pour cette regle : toute IRI ou toute recherche d'instantane qui a
// besoin du nom d'un cluster workload DOIT passer par cette fonction, jamais
// reconstruire "ns/nom" a la main ni utiliser le nom nu du Cluster CAPI seul.
//
// Le cluster de management N'EST PAS decrit ainsi : il garde son nom logique
// propre (--mgmt-name), fixe par l'operateur, independant de ce calcul.
func CanonicalCluster(namespace, name string) string { return namespace + "/" + name }

// SplitCanonicalCluster : inverse de CanonicalCluster. Coupe au PREMIER "/"
// (un nom Kubernetes DNS-1123 ne contient jamais "/", donc sans ambiguite).
// ok=false si s ne contient aucun "/" (nom nu, pas encore canonique).
func SplitCanonicalCluster(s string) (namespace, name string, ok bool) {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// Resource : IRI d'une ressource. group vide = ressource native (core, apps).
func Resource(cluster, namespace, kind, group, name string) string {
	ns := namespace
	if ns == "" {
		ns = NoNS
	}
	k := kind
	if group != "" && !NativeGroup(group) {
		k = kind + "." + group
	}
	return Base + segment(cluster) + "/" + segment(ns) + "/" + segment(k) + "/" + segment(name)
}

// NativeGroup : groupes dont les Kinds restent sans suffixe (decision D1-bis).
func NativeGroup(group string) bool {
	switch group {
	case "", "core", "apps":
		return true
	}
	return false
}

// LabelOf : IRI d'un label observe sur un objet.
func LabelOf(objectIRI, key string) string { return objectIRI + "/label/" + segment(key) }

// NodeSelectorOf : IRI d'une exigence de placement declaree dans Git.
func NodeSelectorOf(deploymentIRI, key string) string {
	return deploymentIRI + "/nodeSelector/" + segment(key)
}

// NetworkFunction : IRI d'une NF.
func NetworkFunction(cluster, namespace, nf string) string {
	return Base + segment(cluster) + "/" + segment(namespace) + "/NetworkFunction/" + segment(nf)
}

// Interface : IRI d'une interface de NF.
func Interface(nfIRI, name string) string { return nfIRI + "/interface/" + segment(name) }

// PLMN : IRI globale d'un PLMN.
func PLMN(mcc, mnc string) string { return Base + "_/PLMN/" + segment(mcc) + "-" + segment(mnc) }

// Slice : IRI globale d'une tranche (S-NSSAI).
func Slice(sst int, sd string) string {
	s := Base + "_/Slice/sst-" + itoa(sst)
	if sd != "" {
		s += "-sd-" + segment(strings.ToLower(sd))
	}
	return s
}

// Scope : IRI d'une portee d'observation.
func Scope(cluster, kind, namespace string) string {
	ns := namespace
	if ns == "" || ns == "*" {
		ns = "_all"
	}
	return Cluster(cluster) + "/_/scope/" + segment(kind) + "/" + segment(ns)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
