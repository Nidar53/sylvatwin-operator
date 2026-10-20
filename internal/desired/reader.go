/*
Package desired : lecture du cote DESIRED (le "depot Git").

Identique en intention a sync_desired() de fact_writer.py v1, avec une
difference majeure : ici la lecture a lieu a CHAQUE reconciliation, pas
seulement au demarrage. Une modification d'un YAML est donc prise en compte,
ce qui supprime les fausses derives de la v1.
*/
package desired

import (
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

// Deployment : la projection minimale dont la TBox a besoin.
type Deployment struct {
	Name         string
	Namespace    string
	Replicas     int
	NodeSelector map[string]string
}

// Read parcourt dir/*.yaml et *.yml et retourne les Deployments declares.
// Un dossier absent n'est pas une erreur : le cote desired est alors vide.
func Read(dir string) ([]Deployment, error) {
	if dir == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var paths []string
	for _, ext := range []string{"*.yaml", "*.yml"} {
		m, err := filepath.Glob(filepath.Join(dir, ext))
		if err != nil {
			return nil, err
		}
		paths = append(paths, m...)
	}

	var out []Deployment
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		// Gere les fichiers multi-documents separes par ---
		for _, doc := range strings.Split(string(raw), "\n---") {
			if strings.TrimSpace(doc) == "" {
				continue
			}
			var dep appsv1.Deployment
			if err := yaml.Unmarshal([]byte(doc), &dep); err != nil {
				continue // hors perimetre v0 : on ignore silencieusement
			}
			if dep.Kind != "Deployment" || dep.Name == "" {
				continue
			}
			replicas := 1
			if dep.Spec.Replicas != nil {
				replicas = int(*dep.Spec.Replicas)
			}
			out = append(out, Deployment{
				Name:         dep.Name,
				Namespace:    dep.Namespace,
				Replicas:     replicas,
				NodeSelector: dep.Spec.Template.Spec.NodeSelector,
			})
		}
	}
	return out, nil
}
