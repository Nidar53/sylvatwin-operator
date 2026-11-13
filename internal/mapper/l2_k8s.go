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
			if start := str(o.Object, "status", "startTime"); start != "" {
				if t, err := time.Parse(time.RFC3339, start); err == nil {
					g.Add(p, rdf.C("startedAt"), rdf.DateTime(t))
				}
			}
		case "Secret":
			// METADONNEES SEULEMENT : on n'accede jamais a o.Object["data"].
			sec := identity(g, s.Cluster, o, at)
			g.Add(sec, rdf.Type, rdf.C("Secret"))
			labels(g, sec, o.GetLabels())
		case "ConfigMap":
			// Ici (L2) : seulement l'existence et la derniere
			// modification (managedFields), pour StaleConfigDrift (D4). Le CONTENU
			// (data) d'une ConfigMap est lu ailleurs, par l'adaptateur L4 (l4_nf.go) :
			// c'est la configuration des NF, qui n'est pas secrete. Un Secret, lui,
			// n'est jamais lu que par ses metadonnees (voir plus haut).
			cm := identity(g, s.Cluster, o, at)
			g.Add(cm, rdf.Type, rdf.C("ConfigMap"))
			if mt := lastManagedFieldTime(o); !mt.IsZero() {
				g.Add(cm, rdf.C("lastModifiedAt"), rdf.DateTime(mt))
			}
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
	refs := secretRefInfo(o.Object, "spec", "template", "spec")
	names := make([]string, 0, len(refs))
	for sn := range refs {
		names = append(names, sn)
	}
	sort.Strings(names)
	for _, sn := range names {
		sec := rdf.IRI(iri.Resource(cluster, o.GetNamespace(), "Secret", "", sn))
		g.Add(d, rdf.C("referencesSecret"), sec)
		// Fait : TOUTES les references a ce Secret sont optionnelles (optional: true).
		// La regle BrokenReference ne le lit pas (a decider, pas ici).
		if refs[sn] {
			g.Add(d, rdf.C("referencesSecretOptionally"), sec)
		}
	}
	for _, cn := range configMapRefs(o.Object) {
		g.Add(d, rdf.C("mountsConfigMap"), rdf.IRI(iri.Resource(cluster, o.GetNamespace(), "ConfigMap", "", cn)))
	}
	// progressDeadlineSeconds : defaut Kubernetes 600 s si absent (D4/horizon).
	pd, ok := i64(o.Object, "spec", "progressDeadlineSeconds")
	if !ok {
		pd = 600
	}
	g.Add(d, rdf.C("progressDeadlineSeconds"), rdf.Dec(float64(pd)))
}

// configMapRefs : ConfigMaps montees en volume par le pod template (base de D4 ;
// on se limite aux volumes, comme pour les Secrets de reference).
func configMapRefs(obj map[string]any) []string {
	set := map[string]bool{}
	ps, _, _ := unstructured.NestedMap(obj, "spec", "template", "spec")
	vols, _, _ := unstructured.NestedSlice(ps, "volumes")
	for _, v := range vols {
		if m, ok := v.(map[string]any); ok {
			if n := str(m, "configMap", "name"); n != "" {
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

// lastManagedFieldTime : le plus recent champ "time" de metadata.managedFields,
// tous gestionnaires confondus (fait brut : "quand" la ConfigMap a change pour
// la derniere fois, pas "qui" ni "quoi" exactement).
func lastManagedFieldTime(o *unstructured.Unstructured) time.Time {
	mf, _, _ := unstructured.NestedSlice(o.Object, "metadata", "managedFields")
	var max time.Time
	for _, e := range mf {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		ts, _ := m["time"].(string)
		if ts == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil && t.After(max) {
			max = t
		}
	}
	return max
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

// secretRefInfo : Secrets references par un pod template (volumes, env, envFrom,
// imagePullSecrets ; conteneurs et init-conteneurs). Lecture de la SPEC du
// Deployment, jamais du Secret. Valeur : true si TOUTES les references a ce
// Secret sont optionnelles (optional: true ; imagePullSecrets ne l'est jamais).
func secretRefInfo(obj map[string]any, podSpecPath ...string) map[string]bool {
	set := map[string]bool{}
	add := func(name string, optional bool) {
		if name == "" {
			return
		}
		if prev, seen := set[name]; seen {
			set[name] = prev && optional
		} else {
			set[name] = optional
		}
	}
	opt := func(m map[string]any, path ...string) bool { b, _ := boolean(m, path...); return b }
	ps, _, _ := unstructured.NestedMap(obj, podSpecPath...)
	vols, _, _ := unstructured.NestedSlice(ps, "volumes")
	for _, v := range vols {
		if m, ok := v.(map[string]any); ok {
			add(str(m, "secret", "secretName"), opt(m, "secret", "optional"))
			srcs, _, _ := unstructured.NestedSlice(m, "projected", "sources")
			for _, sr := range srcs {
				if sm, ok := sr.(map[string]any); ok {
					add(str(sm, "secret", "name"), opt(sm, "secret", "optional"))
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
					add(str(em, "valueFrom", "secretKeyRef", "name"), opt(em, "valueFrom", "secretKeyRef", "optional"))
				}
			}
			efs, _, _ := unstructured.NestedSlice(cm, "envFrom")
			for _, e := range efs {
				if em, ok := e.(map[string]any); ok {
					add(str(em, "secretRef", "name"), opt(em, "secretRef", "optional"))
				}
			}
		}
	}
	ips, _, _ := unstructured.NestedSlice(ps, "imagePullSecrets")
	for _, p := range ips {
		if pm, ok := p.(map[string]any); ok {
			n, _ := pm["name"].(string)
			add(n, false)
		}
	}
	return set
}

// secretRefs : noms des Secrets references, tries.
func secretRefs(obj map[string]any, podSpecPath ...string) []string {
	set := secretRefInfo(obj, podSpecPath...)
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
