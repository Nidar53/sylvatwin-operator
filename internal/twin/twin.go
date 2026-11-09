/*
Package twin : UN cycle du jumeau = greffier puis juge.

	sources (FactSource par cluster) -> Snapshots
	desired (Git @ sha de l'artefact Flux + kustomize) pour chaque Kustomization suivie
	mapper.Build -> graphe des FAITS
	judge.Pipeline.Run -> derives, cycle de vie, verdicts (regles de rules/)
	ecriture : facts.ttl, drifts.nt, verdicts.nt (+ un journal JSON du cycle)

Utilise par le mode --once et par le controleur continu.
*/
package twin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/orange/sylvatwin-operator/internal/desired"
	"github.com/orange/sylvatwin-operator/internal/judge"
	"github.com/orange/sylvatwin-operator/internal/mapper"
	"github.com/orange/sylvatwin-operator/internal/source"
)

// Tracked : une Kustomization Flux dont on suit le cote desire.
type Tracked struct {
	Namespace string // namespace de la Kustomization (management)
	Name      string
}

// Twin : configuration d'un jumeau.
type Twin struct {
	Management     *rest.Config // identite locale sur le management (lecture)
	ManagementName string       // nom du cluster de management (= Cluster CAPI)
	Sources        []source.FactSource
	Tracked        []Tracked
	CacheDir       string // clones Git
	OutDir         string
	Judge          *judge.Pipeline // nil : greffier seul
	// Discovered : clusters CAPI decouverts (convention <cluster>-kubeconfig).
	Discovered []source.Discovered
	// KubeconfigTargets : correspondances declarees ns/secret -> cluster
	// (aucun Secret n'est lu pour les deduire).
	KubeconfigTargets map[string]string
}

// CycleReport : journal d'un cycle (ecrit en JSON, sert a l'evaluation).
type CycleReport struct {
	Start     time.Time         `json:"start"`
	End       time.Time         `json:"end"`
	Clusters  map[string]string `json:"clusters"` // nom -> statut d'observation
	Objects   map[string]int    `json:"objects"`
	Desired   []DesiredReport   `json:"desired"`
	Facts     int               `json:"facts"`
	Drifts    map[string]int    `json:"drifts"`   // classe -> nombre
	Verdicts  map[string]int    `json:"verdicts"` // classe -> nombre
	DriftList []string          `json:"driftList"`
	Verdict   []string          `json:"verdictList"`
	TimingsMS map[string]int64  `json:"timingsMs"`
	Errors    []string          `json:"errors,omitempty"`
}

// DesiredReport : etat du cote desire pour une Kustomization.
type DesiredReport struct {
	Kustomization string `json:"kustomization"`
	Revision      string `json:"revision"`
	Known         bool   `json:"known"`
	Reason        string `json:"reason,omitempty"`
	Objects       int    `json:"objects"`
}

// Cycle : une synchronisation complete.
func (t *Twin) Cycle(ctx context.Context) (*CycleReport, error) {
	rep := &CycleReport{Start: time.Now().UTC(), Clusters: map[string]string{}, Objects: map[string]int{},
		Drifts: map[string]int{}, Verdicts: map[string]int{}, TimingsMS: map[string]int64{}}
	t0 := time.Now()
	var snaps []*mapper.Snapshot
	endpoints := map[string]string{}
	var mgmtSnap *mapper.Snapshot
	for _, s := range t.Sources {
		sn := s.Snapshot(ctx)
		snaps = append(snaps, sn)
		rep.Clusters[sn.Cluster] = sn.Status
		rep.Objects[sn.Cluster] = len(sn.Objects)
		if sn.Endpoint != "" {
			endpoints[sn.Endpoint] = sn.Cluster
		}
		if sn.Cluster == t.ManagementName {
			mgmtSnap = sn
		}
	}
	rep.TimingsMS["observe"] = time.Since(t0).Milliseconds()

	t0 = time.Now()
	fc := source.FluxContext(mgmtSnap, t.Discovered, t.KubeconfigTargets)
	var des []*mapper.Desired
	for _, tr := range t.Tracked {
		d, dr := t.desired(ctx, mgmtSnap, fc, tr)
		rep.Desired = append(rep.Desired, dr)
		if d != nil {
			des = append(des, d)
		}
		if !dr.Known && dr.Reason != "" {
			rep.Errors = append(rep.Errors, "desire "+dr.Kustomization+" : "+dr.Reason)
		}
	}
	rep.TimingsMS["desired"] = time.Since(t0).Milliseconds()

	t0 = time.Now()
	g := mapper.Build(mapper.Input{Snapshots: snaps, Desired: des, Flux: fc})
	rep.Facts = g.Len()
	var nt bytes.Buffer
	if err := g.WriteNTriples(&nt); err != nil {
		return rep, err
	}
	if err := os.MkdirAll(t.OutDir, 0o755); err != nil {
		return rep, err
	}
	if err := writeAtomic(filepath.Join(t.OutDir, "facts.nt"), nt.Bytes()); err != nil {
		return rep, err
	}
	var ttl bytes.Buffer
	_ = g.WriteTurtle(&ttl, "SylvaTwin — faits observes (greffier), "+rep.Start.Format(time.RFC3339))
	_ = writeAtomic(filepath.Join(t.OutDir, "facts.ttl"), ttl.Bytes())
	rep.TimingsMS["map"] = time.Since(t0).Milliseconds()

	if t.Judge != nil {
		res, err := t.Judge.Run(ctx, nt.Bytes(), rep.Start)
		if err != nil {
			rep.Errors = append(rep.Errors, "juge : "+err.Error())
			return rep, err
		}
		for k, v := range res.Timings {
			rep.TimingsMS["judge_"+k] = v.Milliseconds()
		}
		_ = writeAtomic(filepath.Join(t.OutDir, "drifts.nt"), res.Drifts)
		_ = writeAtomic(filepath.Join(t.OutDir, "verdicts.nt"), res.Verdicts)
		rep.Drifts, rep.DriftList = classes(res.Drifts, "https://w3id.org/sylvatwin/drift/")
		rep.Verdicts, rep.Verdict = classes(res.Verdicts, "https://w3id.org/sylvatwin/verdict/")
	}
	rep.End = time.Now().UTC()
	b, _ := json.MarshalIndent(rep, "", "  ")
	_ = writeAtomic(filepath.Join(t.OutDir, "cycle.json"), b)
	return rep, nil
}

// desired : construit le cote desire d'une Kustomization suivie.
func (t *Twin) desired(ctx context.Context, mg *mapper.Snapshot, fc mapper.FluxContext, tr Tracked) (*mapper.Desired, DesiredReport) {
	dr := DesiredReport{Kustomization: tr.Namespace + "/" + tr.Name}
	if mg == nil {
		dr.Reason = "management non observe"
		return nil, dr
	}
	var ks, gr *unstructured.Unstructured
	for _, o := range mg.Objects {
		if o.GetKind() == "Kustomization" && o.GetNamespace() == tr.Namespace && o.GetName() == tr.Name {
			ks = o
		}
	}
	if ks == nil {
		dr.Reason = "Kustomization introuvable"
		return nil, dr
	}
	sns, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "namespace")
	if sns == "" {
		sns = tr.Namespace
	}
	sn, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
	for _, o := range mg.Objects {
		if o.GetKind() == "GitRepository" && o.GetNamespace() == sns && o.GetName() == sn {
			gr = o
		}
	}
	if gr == nil {
		dr.Reason = "GitRepository introuvable"
		return nil, dr
	}
	src := desired.Source{}
	src.URL, _, _ = unstructured.NestedString(gr.Object, "spec", "url")
	src.Branch, _, _ = unstructured.NestedString(gr.Object, "spec", "ref", "branch")
	src.Revision, _, _ = unstructured.NestedString(gr.Object, "status", "artifact", "revision")
	src.Path, _, _ = unstructured.NestedString(ks.Object, "spec", "path")
	src.TargetNamespace, _, _ = unstructured.NestedString(ks.Object, "spec", "targetNamespace")
	dr.Revision = src.Revision
	kcName, _, _ := unstructured.NestedString(ks.Object, "spec", "kubeConfig", "secretRef", "name")
	target := t.ManagementName
	if kcName != "" {
		target = fc.KubeconfigTargets[tr.Namespace+"/"+kcName]
	}
	if target == "" {
		dr.Reason = "cluster cible inconnu"
		return nil, dr
	}
	res, err := desired.Build(ctx, t.CacheDir, src)
	if err != nil {
		dr.Reason = err.Error()
		return &mapper.Desired{Revision: src.Revision, Known: false, TargetCluster: target}, dr
	}
	dr.Known, dr.Reason, dr.Objects = res.Known, res.Reason, len(res.Objects)
	return &mapper.Desired{Revision: res.Revision, Known: res.Known, TargetCluster: target, Objects: res.Objects}, dr
}

// classes : comptage par classe et liste "Classe <iri courte>" (rdf:type seulement).
func classes(nt []byte, base string) (map[string]int, []string) {
	count := map[string]int{}
	var list []string
	for _, l := range strings.Split(string(nt), "\n") {
		f := strings.Fields(l)
		if len(f) < 3 || f[1] != "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>" {
			continue
		}
		c := strings.TrimSuffix(strings.TrimPrefix(f[2], "<https://w3id.org/sylvatwin/core#"), ">")
		count[c]++
		list = append(list, c+" "+strings.TrimSuffix(strings.TrimPrefix(f[0], "<"+base), ">"))
	}
	sort.Strings(list)
	return count, list
}

func writeAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), path)
}

// Summary : une ligne lisible pour les logs (aucune donnee sensible).
func (r *CycleReport) Summary() string {
	return fmt.Sprintf("clusters=%v faits=%d derives=%v verdicts=%v duree=%s", r.Clusters, r.Facts, r.Drifts, r.Verdicts, r.End.Sub(r.Start).Round(time.Millisecond))
}
