package mapper

import (
	"fmt"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// mapCluster : le cluster reel (KubernetesCluster) et ses faits d'observation (P3).
// Attention : PAS de clusterName/uid/observedAt sur un KubernetesCluster (domaine
// KubernetesResource, disjointe) : son nom va dans rdfs:label.
func mapCluster(g *rdf.Graph, s *Snapshot) {
	kc := clusterIRI(s.Cluster)
	g.Add(kc, rdf.Type, rdf.C("KubernetesCluster"))
	g.Add(kc, rdf.Label, rdf.Str(s.Cluster))
	if s.Status != "" {
		g.Add(kc, rdf.C("observationStatus"), rdf.Str(s.Status))
	}
	if !s.LastSync.IsZero() {
		g.Add(kc, rdf.C("lastSyncTime"), rdf.DateTime(s.LastSync))
	}
	// Watch temoin et fraicheur (partie 2) : u_c(t), theta.
	if s.WitnessLeaseName != "" {
		g.Add(kc, rdf.C("witnessLeaseName"), rdf.Str(s.WitnessLeaseName))
	}
	if !s.LastWitnessRenew.IsZero() {
		g.Add(kc, rdf.C("lastWitnessRenewTime"), rdf.DateTime(s.LastWitnessRenew))
	}
	if s.FreshnessHorizon > 0 {
		g.Add(kc, rdf.C("freshnessHorizon"), rdf.Dec(s.FreshnessHorizon))
	}
	for _, sc := range s.Scopes {
		ns := sc.Namespace
		if ns == "" {
			ns = "*"
		}
		si := rdf.IRI(iri.Scope(s.Cluster, sc.Kind, ns))
		g.Add(kc, rdf.C("hasObservationScope"), si)
		g.Add(si, rdf.Type, rdf.C("ObservationScope"))
		g.Add(si, rdf.C("scopeKind"), rdf.Str(sc.Kind))
		g.Add(si, rdf.C("scopeNamespace"), rdf.Str(ns))
	}
}

// mapL1 : Cluster et Machine (CAPI), observes dans le cluster de management.
// endpoints : "host:port" -> nom du cluster reel observe a cette adresse (pour
// clusterDescribedBy, ecrit sur le cluster REEL, ET pour machineHasNode : voir
// resolveDescribedClusterName ci-dessous).
func mapL1(g *rdf.Graph, s *Snapshot, endpoints map[string]string) {
	at := s.ObservedAt
	// 1ere passe : "namespace/nom" du Cluster CAPI -> son "host:port". Sert a
	// resoudre machineHasNode (2e passe) vers le nom REEL du FactSource qui
	// decrit ce cluster (comme clusterDescribedBy), PAS un calcul aveugle
	// depuis namespace/nom : le cluster de MANAGEMENT lui-meme est decrit par
	// un Cluster CAPI (ex. "sylva-system/management-cluster") dont le nom
	// canonique NE correspond PAS a son nom logique reel (--mgmt-name,
	// independant, fourni par l'utilisateur) — contrairement a un cluster
	// workload, ou namespace/nom EST la forme que le provider cluster-api de
	// multicluster-runtime utilise pour l'engager.
	capiEndpoint := map[string]string{} // "ns/nom" -> "host:port"
	for _, o := range s.Objects {
		if groupOf(o) != "cluster.x-k8s.io" || kindOf(o) != "Cluster" {
			continue
		}
		host := str(o.Object, "spec", "controlPlaneEndpoint", "host")
		if host == "" {
			continue
		}
		port, _ := i64(o.Object, "spec", "controlPlaneEndpoint", "port")
		if port == 0 {
			port = 6443
		}
		capiEndpoint[o.GetNamespace()+"/"+o.GetName()] = fmt.Sprintf("%s:%d", host, port)
	}
	for _, o := range s.Objects {
		if groupOf(o) != "cluster.x-k8s.io" {
			continue
		}
		switch kindOf(o) {
		case "Cluster":
			c := identity(g, s.Cluster, o, at)
			g.Add(c, rdf.Type, rdf.C("Cluster"))
			g.Add(c, rdf.C("apiGroup"), rdf.Str("cluster.x-k8s.io"))
			// v1beta2 : condition "Available" (pas de "Ready" sur Cluster)
			if st := condition(o.Object, "Available"); st != "" {
				g.Add(c, rdf.C("ready"), rdf.Bool(st == "True"))
			}
			managedByLabels(g, s.Cluster, c, o.GetNamespace(), o.GetLabels())
			if ep, ok := capiEndpoint[o.GetNamespace()+"/"+o.GetName()]; ok {
				if real, ok := endpoints[ep]; ok {
					g.Add(clusterIRI(real), rdf.C("clusterDescribedBy"), c)
				}
			}
		case "Machine":
			m := identity(g, s.Cluster, o, at)
			g.Add(m, rdf.Type, rdf.C("Machine"))
			g.Add(m, rdf.C("apiGroup"), rdf.Str("cluster.x-k8s.io"))
			if cn := str(o.Object, "spec", "clusterName"); cn != "" {
				g.Add(m, rdf.C("machineOfCluster"),
					rdf.IRI(iri.Resource(s.Cluster, o.GetNamespace(), "Cluster", "cluster.x-k8s.io", cn)))
				// Le Node vit dans le cluster DECRIT : son nom logique est celui du
				// FactSource REEL qui l'observe, resolu par endpoint (comme
				// clusterDescribedBy ci-dessus) — JAMAIS un calcul aveugle depuis
				// namespace/nom (voir resolveDescribedClusterName). Si l'endpoint ne
				// correspond a AUCUN FactSource actuellement engage (ex. cluster
				// workload pas encore observe ce cycle), on replie sur le nom
				// CANONIQUE (meilleure estimation disponible, coherente avec
				// iri.CanonicalCluster utilise ailleurs pour ce meme cluster), sauf
				// quand le Cluster CAPI decrit est celui du management LUI-MEME
				// (son host:port == celui du Snapshot courant) : dans ce cas precis
				// on connait le nom reel avec certitude (s.Cluster), meme sans table
				// d'endpoints, et on ne doit PAS utiliser le nom canonique (qui
				// serait faux : --mgmt-name est independant de "namespace/nom").
				if node := str(o.Object, "status", "nodeRef", "name"); node != "" {
					wl := resolveDescribedClusterName(s, o.GetNamespace(), cn, capiEndpoint, endpoints)
					g.Add(m, rdf.C("machineHasNode"), rdf.IRI(iri.Resource(wl, "", "Node", "", node)))
				}
			}
			if ph := str(o.Object, "status", "phase"); ph != "" {
				g.Add(m, rdf.C("machinePhase"), rdf.Str(ph))
			}
		}
	}
}

// resolveDescribedClusterName : nom LOGIQUE REEL du cluster decrit par le
// Cluster CAPI (capiNS, capiName), tel qu'un FactSource l'utilise comme
// Snapshot.Cluster. Ordre de resolution :
//  1. l'endpoint du Cluster CAPI correspond a celui du Snapshot COURANT (s) :
//     le Cluster CAPI decrit s LUI-MEME (cas du management, dont le nom
//     logique --mgmt-name est independant de "namespace/nom") -> s.Cluster ;
//  2. l'endpoint correspond a un AUTRE FactSource deja connu (table
//     endpoints, alimentee par tous les Snapshots du cycle, meme mecanisme
//     que clusterDescribedBy) -> ce nom-la ;
//  3. aucune correspondance (cluster pas encore engage/observe ce cycle) :
//     repli sur le nom CANONIQUE (iri.CanonicalCluster(capiNS, capiName)),
//     qui EST la forme reelle pour tout cluster WORKLOAD (convention du
//     provider cluster-api de multicluster-runtime), donc une estimation
//     fiable dans ce cas (jamais utilisee pour le management, deja couvert
//     par le cas 1).
func resolveDescribedClusterName(s *Snapshot, capiNS, capiName string, capiEndpoint, endpoints map[string]string) string {
	ep, hasEP := capiEndpoint[capiNS+"/"+capiName]
	if hasEP && ep == s.Endpoint {
		return s.Cluster
	}
	if hasEP {
		if real, ok := endpoints[ep]; ok {
			return real
		}
	}
	return iri.CanonicalCluster(capiNS, capiName)
}
