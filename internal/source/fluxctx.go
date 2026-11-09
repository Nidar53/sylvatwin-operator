package source

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/orange/sylvatwin-operator/internal/mapper"
)

// FluxContext : informations dont le mapper Flux a besoin et qu'il ne peut pas
// lire lui-meme sans toucher a un Secret.
//
//   - KubeconfigTargets : pour chaque Kustomization/HelmRelease avec
//     spec.kubeConfig.secretRef, on lit le Secret EN MEMOIRE, on n'en garde QUE
//     le champ server (host:port), compare aux endpoints connus. Rien d'autre
//     n'est conserve ; le Secret n'est ni journalise ni ecrit.
//   - HelmChartRevisions : status.artifact.revision des HelmChart (non secret).
func FluxContext(ctx context.Context, mgmt *rest.Config, s *mapper.Snapshot, endpoints map[string]string) mapper.FluxContext {
	fc := mapper.FluxContext{KubeconfigTargets: map[string]string{}, HelmChartRevisions: map[string]string{}}
	if s == nil {
		return fc
	}
	cs, err := kubernetes.NewForConfig(mgmt)
	if err != nil {
		return fc
	}
	seen := map[string]bool{}
	for _, o := range s.Objects {
		if o.GetKind() == "HelmChart" {
			if rev, _, _ := unstructured.NestedString(o.Object, "status", "artifact", "revision"); rev != "" {
				fc.HelmChartRevisions[o.GetNamespace()+"/"+o.GetName()] = rev
			}
			continue
		}
		name, _, _ := unstructured.NestedString(o.Object, "spec", "kubeConfig", "secretRef", "name")
		if name == "" {
			continue
		}
		key := o.GetNamespace() + "/" + name
		if seen[key] {
			continue
		}
		seen[key] = true
		sec, err := cs.CoreV1().Secrets(o.GetNamespace()).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue // pas de cible connue : targetsCluster ne sera pas ecrit
		}
		k, _, _ := unstructured.NestedString(o.Object, "spec", "kubeConfig", "secretRef", "key")
		raw := sec.Data[k]
		if len(raw) == 0 {
			raw = sec.Data["value"]
		}
		if len(raw) == 0 {
			raw = sec.Data["value.yaml"]
		}
		server := serverOnly(raw)
		for i := range raw {
			raw[i] = 0
		}
		for k2 := range sec.Data {
			delete(sec.Data, k2)
		}
		if c, ok := endpoints[hostPort(server)]; ok {
			fc.KubeconfigTargets[key] = c
		}
	}
	return fc
}

// serverOnly : extrait le seul champ server du contexte courant d'un kubeconfig.
func serverOnly(raw []byte) string {
	cfg, err := clientcmd.Load(raw)
	if err != nil || cfg == nil {
		return ""
	}
	ctxName := cfg.CurrentContext
	c, ok := cfg.Contexts[ctxName]
	if !ok {
		for _, cl := range cfg.Clusters {
			return cl.Server
		}
		return ""
	}
	if cl, ok := cfg.Clusters[c.Cluster]; ok {
		return cl.Server
	}
	return ""
}
