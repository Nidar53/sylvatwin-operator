package mapper

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// kindOf : Kind de l'objet. Les Secrets observes en metadonnees seulement sont
// des PartialObjectMetadata : le Kind reel est porte par x-sylvatwin-kind (photo
// de test) ou par l'appelant (source live).
func kindOf(o *unstructured.Unstructured) string {
	if k, ok := o.Object["metadata"].(map[string]any)["x-sylvatwin-kind"].(string); ok && k != "" {
		return k
	}
	return o.GetKind()
}

func groupOf(o *unstructured.Unstructured) string {
	// PartialObjectMetadata (Secret en metadonnees seulement) : apiVersion
	// meta.k8s.io/v1, mais l'objet reel est du groupe core (Secret).
	if o.GetKind() == "PartialObjectMetadata" {
		if k := kindOf(o); k == "Secret" || k == "ConfigMap" {
			return ""
		}
	}
	gv, err := schema.ParseGroupVersion(o.GetAPIVersion())
	if err != nil {
		return ""
	}
	return gv.Group
}

// resIRI : IRI d'un objet observe dans un cluster.
func resIRI(cluster string, o *unstructured.Unstructured) rdf.Term {
	return rdf.IRI(iri.Resource(cluster, o.GetNamespace(), kindOf(o), groupOf(o), o.GetName()))
}

func nsIRI(cluster, ns string) rdf.Term {
	return rdf.IRI(iri.Resource(cluster, "", "Namespace", "", ns))
}

func clusterIRI(cluster string) rdf.Term { return rdf.IRI(iri.Cluster(cluster)) }

// identity : faits communs a toute KubernetesResource observee.
func identity(g *rdf.Graph, cluster string, o *unstructured.Unstructured, at time.Time) rdf.Term {
	s := resIRI(cluster, o)
	g.Add(s, rdf.C("clusterName"), rdf.Str(cluster))
	g.Add(s, rdf.C("resourceName"), rdf.Str(o.GetName()))
	g.Add(s, rdf.C("resourceInCluster"), clusterIRI(cluster))
	if uid := string(o.GetUID()); uid != "" {
		g.Add(s, rdf.C("uid"), rdf.Str(uid))
	}
	if ns := o.GetNamespace(); ns != "" {
		g.Add(s, rdf.C("inNamespace"), nsIRI(cluster, ns))
	}
	if !at.IsZero() {
		g.Add(s, rdf.C("observedAt"), rdf.DateTime(at))
	}
	return s
}

func str(o map[string]any, path ...string) string {
	v, _, _ := unstructured.NestedString(o, path...)
	return v
}

func i64(o map[string]any, path ...string) (int64, bool) {
	v, ok, _ := unstructured.NestedFieldNoCopy(o, path...)
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func boolean(o map[string]any, path ...string) (bool, bool) {
	v, ok, _ := unstructured.NestedBool(o, path...)
	return v, ok
}

// condition : status d'une condition (True/False/Unknown), "" si absente.
func condition(o map[string]any, typ string) string {
	conds, _, _ := unstructured.NestedSlice(o, "status", "conditions")
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if ok && m["type"] == typ {
			s, _ := m["status"].(string)
			return s
		}
	}
	return ""
}

// seconds : duree Go ("2m", "24h0m0s", "60s") en secondes ; ok=false si vide.
func seconds(d string) (float64, bool) {
	d = strings.TrimSpace(d)
	if d == "" {
		return 0, false
	}
	v, err := time.ParseDuration(d)
	if err != nil {
		return 0, false
	}
	return v.Seconds(), true
}

// endpointHostPort : "host:port" d'une URL de serveur Kubernetes.
func endpointHostPort(server string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(server, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if !strings.Contains(s, ":") {
		s += ":443"
	}
	return s
}
