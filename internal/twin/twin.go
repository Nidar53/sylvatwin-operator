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
	// Memo : memoire du dernier cycle (cote desire + flux), partagee entre les
	// Twin successifs du Watcher. Permet Rejudge sans relire le cluster ni Git.
	Memo *Memo
	// Cause : ce qui a declenche le cycle (journal, A7).
	Cause string
}

// Memo : entrees du mapper conservees d'un cycle a l'autre (instantanes inclus
// par les sources). Pas de verrou : seul l'executeur unique l'utilise.
type Memo struct {
	Des []*mapper.Desired
	FC  mapper.FluxContext
	OK  bool
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
	// FactsBytes : taille de l'ABox (faits) en N-Triples.
	FactsBytes int `json:"factsBytes"`
	// APIRequests : requetes API Kubernetes de ce cycle, par cluster (0 pour une echeance).
	APIRequests map[string]int64 `json:"apiRequests"`
	// DriftsByVerdict : classe de derive -> verdict -> nombre. Les causes racines
	// et les impacts (FunctionImpactDrift) sont des classes distinctes.
	DriftsByVerdict map[string]map[string]int `json:"driftsByVerdict"`
	// Cause : evenement | echeance | reprise (A7). Une echeance ne relit pas le cluster.
	Cause string `json:"cause,omitempty"`
	// NextDeadline : prochaine echeance utile calculee par SPARQL (zero = aucune).
	NextDeadline time.Time `json:"nextDeadline,omitempty"`
	Errors       []string  `json:"errors,omitempty"`
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
	rep := &CycleReport{Cause: t.Cause, Start: time.Now().UTC(), Clusters: map[string]string{}, Objects: map[string]int{},
		Drifts: map[string]int{}, Verdicts: map[string]int{}, TimingsMS: map[string]int64{}, APIRequests: map[string]int64{}}
	t0 := time.Now()
	var snaps []*mapper.Snapshot
	endpoints := map[string]string{}
	var mgmtSnap *mapper.Snapshot
	for _, s := range t.Sources {
		sn := s.Snapshot(ctx)
		if c, ok := s.(interface{ LastAPIRequests() int64 }); ok {
			rep.APIRequests[sn.Cluster] = c.LastAPIRequests()
		}
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
	// D1 : Discovered n'est pas toujours fourni explicitement (mode continu,
	// watcher) ; il peut toujours etre deduit du Snapshot du management lui-
	// meme (fonction pure, aucun appel reseau supplementaire). Si l'appelant
	// l'a deja rempli (mode --once, qui l'obtient par un LIST a part), on
	// GARDE sa valeur : elle peut porter des informations plus a jour.
	discovered := t.Discovered
	if discovered == nil {
		discovered = source.DiscoveredFromSnapshot(mgmtSnap)
	}
	// D2 : les cibles DECLAREES (--kubeconfig-target) peuvent etre un nom NU
	// ("my-workload-kubeadm-capd") ou deja la forme canonique ("ns/nom"). On
	// resout les noms nus en cherchant le Cluster CAPI correspondant dans le
	// management ; zero ou plusieurs correspondances -> erreur EXPLICITE dans
	// le rapport, et la cible reste absente de resolvedTargets (le desire sera
	// "inconnu" pour toute Kustomization qui la visait, on n'invente rien).
	resolvedTargets, resolveErrs := source.ResolveTargets(mgmtSnap, t.KubeconfigTargets)
	for _, e := range resolveErrs {
		rep.Errors = append(rep.Errors, "kubeconfig-target "+e)
	}
	fc := source.FluxContext(mgmtSnap, discovered, resolvedTargets)
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
	if t.Memo != nil {
		t.Memo.Des, t.Memo.FC, t.Memo.OK = des, fc, true
	}
	return t.finish(ctx, rep, snaps, des, fc)
}

// Rejudge : rejuge SANS relire le cluster ni Git (reveil par echeance, A3).
// Les instantanes sont ceux du dernier cycle (Cached), le cote desire vient de
// Memo ; seul l'instant du jugement change (et l'age du temoin, lu en memoire).
func (t *Twin) Rejudge(ctx context.Context) (*CycleReport, error) {
	rep := &CycleReport{Cause: t.Cause, Start: time.Now().UTC(), Clusters: map[string]string{}, Objects: map[string]int{},
		Drifts: map[string]int{}, Verdicts: map[string]int{}, TimingsMS: map[string]int64{}, APIRequests: map[string]int64{}}
	if t.Memo == nil || !t.Memo.OK {
		return rep, fmt.Errorf("rejugement impossible : aucun cycle complet precedent")
	}
	var snaps []*mapper.Snapshot
	for _, s := range t.Sources {
		c, ok := s.(interface{ Cached() *mapper.Snapshot })
		if !ok {
			return rep, fmt.Errorf("rejugement impossible : source %s sans cache", s.Name())
		}
		sn := c.Cached()
		if sn == nil {
			return rep, fmt.Errorf("rejugement impossible : source %s jamais lue", s.Name())
		}
		snaps = append(snaps, sn)
		rep.Clusters[sn.Cluster] = sn.Status
		rep.Objects[sn.Cluster] = len(sn.Objects)
	}
	return t.finish(ctx, rep, snaps, t.Memo.Des, t.Memo.FC)
}

// finish : faits (mapper pur) -> juge -> journal. Commun a Cycle et Rejudge.
func (t *Twin) finish(ctx context.Context, rep *CycleReport, snaps []*mapper.Snapshot, des []*mapper.Desired, fc mapper.FluxContext) (*CycleReport, error) {
	t0 := time.Now()
	g := mapper.Build(mapper.Input{Snapshots: snaps, Desired: des, Flux: fc})
	rep.Facts = g.Len()
	var nt bytes.Buffer
	if err := g.WriteNTriples(&nt); err != nil {
		return rep, err
	}
	rep.FactsBytes = nt.Len()
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
		if dl, err := t.Judge.NextDeadline(ctx, rep.Start); err != nil {
			rep.Errors = append(rep.Errors, "echeance : "+err.Error())
		} else {
			rep.NextDeadline = dl
		}
		rep.Drifts, rep.DriftList = classes(res.Drifts, "https://w3id.org/sylvatwin/drift/")
		rep.Verdicts, rep.Verdict = classes(res.Verdicts, "https://w3id.org/sylvatwin/verdict/")
		rep.DriftsByVerdict = byClassAndVerdict(rep.Verdict)
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

// byClassAndVerdict : "<Verdict> <ClasseDeDerive>/..." -> classe -> verdict -> n.
func byClassAndVerdict(list []string) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, e := range list {
		v, rest, ok := strings.Cut(e, " ")
		if !ok {
			continue
		}
		cl, _, _ := strings.Cut(rest, "/")
		if out[cl] == nil {
			out[cl] = map[string]int{}
		}
		out[cl][v]++
	}
	return out
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

// Summary : une ligne lisible pour les logs (aucune donnee sensible). D3 :
// erreursPartielles = len(r.Errors) (desire inconnu, cible --kubeconfig-target
// non resolue...), DISTINCT de l'erreur globale du cycle (celle-ci, si elle
// existe, est journalisee a part par l'appelant : twin.Twin.Cycle la renvoie
// en 2e valeur de retour, jamais dans r.Errors).
func (r *CycleReport) Summary() string {
	return fmt.Sprintf("clusters=%v faits=%d derives=%v verdicts=%v erreursPartielles=%d duree=%s",
		r.Clusters, r.Facts, r.Drifts, r.Verdicts, len(r.Errors), r.End.Sub(r.Start).Round(time.Millisecond))
}
