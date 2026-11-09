package desired

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Result : objets desires rendus, et verdict sur la fiabilite du desire.
type Result struct {
	Revision string // revision Flux (artefact)
	Known    bool   // false : sha clone != sha de l'artefact -> desire inconnu
	Reason   string // explication si Known == false
	Objects  []*unstructured.Unstructured
}

// Build : checkout du sha exact + kustomize build du chemin de la Kustomization.
func Build(ctx context.Context, cacheDir string, src Source) (*Result, error) {
	res := &Result{Revision: src.Revision}
	dir, got, err := Checkout(ctx, cacheDir, src)
	if err != nil {
		return nil, err
	}
	if want := SHA(src.Revision); got != want {
		res.Known = false
		res.Reason = fmt.Sprintf("sha clone %s different du sha de l'artefact Flux %s", got, want)
		return res, nil
	}
	objs, err := Render(filepath.Join(dir, strings.TrimPrefix(src.Path, "./")), src.TargetNamespace)
	if err != nil {
		return nil, err
	}
	res.Known = true
	res.Objects = objs
	return res, nil
}

// Render : kustomize build (bibliotheque krusty, comme kustomize-controller),
// puis application de targetNamespace aux objets namespaces.
func Render(path, targetNamespace string) ([]*unstructured.Unstructured, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	rm, err := k.Run(filesys.MakeFsOnDisk(), path)
	if err != nil {
		return nil, fmt.Errorf("kustomize build %s : %w", path, err)
	}
	var out []*unstructured.Unstructured
	for _, r := range rm.Resources() {
		m, err := r.Map()
		if err != nil {
			return nil, err
		}
		u := &unstructured.Unstructured{Object: m}
		if targetNamespace != "" && !clusterScoped(u.GetKind()) {
			u.SetNamespace(targetNamespace)
		}
		out = append(out, u)
	}
	return out, nil
}

// clusterScoped : Kinds sans namespace presents dans nos manifestes.
func clusterScoped(kind string) bool {
	switch kind {
	case "Namespace", "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition",
		"PersistentVolume", "StorageClass", "Node":
		return true
	}
	return false
}
