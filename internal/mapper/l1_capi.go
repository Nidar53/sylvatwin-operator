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
// clusterDescribedBy, ecrit sur le cluster REEL).
func mapL1(g *rdf.Graph, s *Snapshot, endpoints map[string]string) {
	at := s.ObservedAt
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
			host := str(o.Object, "spec", "controlPlaneEndpoint", "host")
			port, _ := i64(o.Object, "spec", "controlPlaneEndpoint", "port")
			if host != "" {
				if port == 0 {
					port = 6443
				}
				if real, ok := endpoints[fmt.Sprintf("%s:%d", host, port)]; ok {
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
				// Le Node vit dans le cluster DECRIT : son nom logique est le nom
				// CANONIQUE du cluster workload, SOUS LA MEME FORME que celui utilise
				// par le provider cluster-api de multicluster-runtime v0.24.1 pour
				// engager ce cluster ("namespace/nom", cf. iri.CanonicalCluster) —
				// PAS le seul nom nu du Cluster CAPI, qui ne correspond au nom logique
				// du FactSource que par coincidence (quand namespace == nom). Le
				// namespace du Cluster CAPI decrit est celui de la Machine qui le
				// reference (convention CAPI : Machine et Cluster vivent dans le meme
				// namespace).
				if node := str(o.Object, "status", "nodeRef", "name"); node != "" {
					wl := iri.CanonicalCluster(o.GetNamespace(), cn)
					g.Add(m, rdf.C("machineHasNode"), rdf.IRI(iri.Resource(wl, "", "Node", "", node)))
				}
			}
			if ph := str(o.Object, "status", "phase"); ph != "" {
				g.Add(m, rdf.C("machinePhase"), rdf.Str(ph))
			}
		}
	}
}
