package mapper

import (
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// mapL2 : Namespace, Deployment, ReplicaSet, Pod, Node, Secret, Label.
func mapL2(g *rdf.Graph, s *Snapshot) {
	at := s.ObservedAt
	for _, o := range s.Objects {
		switch kindOf(o) {
		case "Namespace":
			identity(g, s.Cluster, o, at)
			g.Add(resIRI(s.Cluster, o), rdf.Type, rdf.C("Namespace"))
		case "Node":
			n := identity(g, s.Cluster, o, at)
			g.Add(n, rdf.Type, rdf.C("Node"))
			labels(g, n, o.GetLabels())
		case "Deployment":
			mapDeployment(g, s.Cluster, o, at)
		case "ReplicaSet":
			rs := identity(g, s.Cluster, o, at)
			g.Add(rs, rdf.Type, rdf.C("ReplicaSet"))
			owners(g, s.Cluster, rs, o)
		case "Pod":
			p := identity(g, s.Cluster, o, at)
			g.Add(p, rdf.Type, rdf.C("Pod"))
			owners(g, s.Cluster, p, o)
			// Pod non planifie (Pending) : pas de runsOn. Il n'est pas mal place,
			// il n'est pas place (invariant repris de la v0).
			if node := str(o.Object, "spec", "nodeName"); node != "" {
				g.Add(p, rdf.C("runsOn"), rdf.IRI(iri.Resource(s.Cluster, "", "Node", "", node)))
			}
			if ph := str(o.Object, "status", "phase"); ph != "" {
				g.Add(p, rdf.C("podPhase"), rdf.Str(ph))
			}
		case "Secret":
			// METADONNEES SEULEMENT : on n'accede jamais a o.Object["data"].
			sec := identity(g, s.Cluster, o, at)
			g.Add(sec, rdf.Type, rdf.C("Secret"))
			labels(g, sec, o.GetLabels())
		}
	}
}

// mapDeployment : cote OBSERVE (spec et status). Le cote desire (Git) est ecrit
// par mapDesired sur la MEME IRI (Option C).
func mapDeployment(g *rdf.Graph, cluster string, o *unstructured.Unstructured, at time.Time) {
	d := identity(g, cluster, o, at)
	g.Add(d, rdf.Type, rdf.C("Deployment"))
	// Valeur 2/3 : spec.replicas du cluster (1 si absent, comme Kubernetes).
	spec, ok := i64(o.Object, "spec", "replicas")
	if !ok {
		spec = 1
	}
	g.Add(d, rdf.C("observedSpecReplicas"), rdf.Int(spec))
	// Valeur 3/3 : status.readyReplicas (0 si absent).
	ready, _ := i64(o.Object, "status", "readyReplicas")
	g.Add(d, rdf.C("actualReplicas"), rdf.Int(ready))
	for _, sn := range secretRefs(o.Object, "spec", "template", "spec") {
		g.Add(d, rdf.C("referencesSecret"), rdf.IRI(iri.Resource(cluster, o.GetNamespace(), "Secret", "", sn)))
	}
}

// owners : copie fidele de metadata.ownerReferences (Pod -> ReplicaSet,
// ReplicaSet -> Deployment, pod statique -> Node...). JAMAIS de repli sur le
// label "app" (decision D2).
func owners(g *rdf.Graph, cluster string, s rdf.Term, o *unstructured.Unstructured) {
	for _, ref := range o.GetOwnerReferences() {
		ns := o.GetNamespace()
		if ref.Kind == "Node" {
			ns = ""
		}
		group := ""
		if gv := ref.APIVersion; gv != "" && gv != "v1" {
			if i := len(gv) - len("/"+lastSeg(gv)); i > 0 {
				group = gv[:i]
			}
		}
		g.Add(s, rdf.C("ownedBy"), rdf.IRI(iri.Resource(cluster, ns, ref.Kind, group, ref.Name)))
	}
}

func lastSeg(gv string) string {
	for i := len(gv) - 1; i >= 0; i-- {
		if gv[i] == '/' {
			return gv[i+1:]
		}
	}
	return gv
}

// labels : chaque label observe devient un individu :Label (hasKey, hasValue).
func labels(g *rdf.Graph, obj rdf.Term, l map[string]string) {
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		li := rdf.IRI(iri.LabelOf(obj.Value(), k))
		g.Add(obj, rdf.C("hasLabel"), li)
		g.Add(li, rdf.Type, rdf.C("Label"))
		g.Add(li, rdf.C("hasKey"), rdf.Str(k))
		g.Add(li, rdf.C("hasValue"), rdf.Str(l[k]))
	}
}

// secretRefs : Secrets references par un pod template (volumes, env, envFrom,
// imagePullSecrets ; conteneurs et init-conteneurs). Lecture de la SPEC du
// Deployment, jamais du Secret.
func secretRefs(obj map[string]any, podSpecPath ...string) []string {
	set := map[string]bool{}
	ps, _, _ := unstructured.NestedMap(obj, podSpecPath...)
	vols, _, _ := unstructured.NestedSlice(ps, "volumes")
	for _, v := range vols {
		if m, ok := v.(map[string]any); ok {
			if n := str(m, "secret", "secretName"); n != "" {
				set[n] = true
			}
			srcs, _, _ := unstructured.NestedSlice(m, "projected", "sources")
			for _, sr := range srcs {
				if sm, ok := sr.(map[string]any); ok {
					if n := str(sm, "secret", "name"); n != "" {
						set[n] = true
					}
				}
			}
		}
	}
	for _, field := range []string{"containers", "initContainers"} {
		cs, _, _ := unstructured.NestedSlice(ps, field)
		for _, c := range cs {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			envs, _, _ := unstructured.NestedSlice(cm, "env")
			for _, e := range envs {
				if em, ok := e.(map[string]any); ok {
					if n := str(em, "valueFrom", "secretKeyRef", "name"); n != "" {
						set[n] = true
					}
				}
			}
			efs, _, _ := unstructured.NestedSlice(cm, "envFrom")
			for _, e := range efs {
				if em, ok := e.(map[string]any); ok {
					if n := str(em, "secretRef", "name"); n != "" {
						set[n] = true
					}
				}
			}
		}
	}
	ips, _, _ := unstructured.NestedSlice(ps, "imagePullSecrets")
	for _, p := range ips {
		if pm, ok := p.(map[string]any); ok {
			if n, _ := pm["name"].(string); n != "" {
				set[n] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
