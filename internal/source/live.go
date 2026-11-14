package source

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"

	"github.com/orange/sylvatwin-operator/internal/mapper"
)

// Live : FactSource par lecture directe de l'API (LIST a chaque Snapshot).
//
// Phase 1 : LIST complet a chaque cycle (baseline "synchro fixe" de la phase 3).
// Les Watch (evenementiel) et l'adaptation du mode d'observation sont la phase 3 ;
// l'interface FactSource ne change pas.
type Live struct {
	name     string
	endpoint string
	specs    []Spec
	dyn      dynamic.Interface
	meta     metadata.Interface
	witness  *Witness // Watch temoin (partie 2), optionnel
	// readTimeout : duree maximale de TOUTES les lectures d'un Snapshot de ce
	// cluster (0 = aucune borne autre que celle du contexte et du client).
	readTimeout time.Duration

	mu   sync.Mutex
	last *mapper.Snapshot // dernier instantane REUSSI

	calls     atomic.Int64 // requetes API du Snapshot en cours
	lastCalls atomic.Int64 // requetes API du dernier Snapshot termine
}

// LastAPIRequests : nombre de requetes API Kubernetes du dernier Snapshot (A7).
func (l *Live) LastAPIRequests() int64 { return l.lastCalls.Load() }

// SetReadTimeout borne la duree totale d'un Snapshot (parametre --read-timeout).
// Une lecture qui depasse est un echec ordinaire : statut "injoignable", dernier
// instantane reussi conserve. A appeler avant le premier Snapshot.
func (l *Live) SetReadTimeout(d time.Duration) { l.readTimeout = d }

// SetWitness attache un Watch temoin (P1). A appeler avant le premier Snapshot.
func (l *Live) SetWitness(w *Witness) { l.witness = w }

// NewLive construit une source a partir d'un rest.Config deja pret (identite et
// impersonation reglees par l'appelant). cfg n'est jamais journalise.
func NewLive(name string, cfg *rest.Config, specs []Spec) (*Live, error) {
	c := rest.CopyConfig(cfg)
	c.QPS, c.Burst = 20, 40
	c.Timeout = 30 * time.Second
	dyn, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	md, err := metadata.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return &Live{name: name, endpoint: hostPort(c.Host), specs: specs, dyn: dyn, meta: md}, nil
}

func hostPort(host string) string {
	u, err := url.Parse(host)
	if err != nil || u.Host == "" {
		return strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	}
	if u.Port() == "" {
		return u.Hostname() + ":443"
	}
	return u.Host
}

func (l *Live) Name() string { return l.name }
func (l *Live) Close()       {}

// Endpoint : "host:port" observe (pour clusterDescribedBy / targetsCluster).
func (l *Live) Endpoint() string { return l.endpoint }

// Snapshot : LIST de chaque type de la portee. Tout ou rien : si une lecture
// echoue, l'instantane precedent est renvoye avec le statut d'echec (P3).
func (l *Live) Snapshot(ctx context.Context) *mapper.Snapshot {
	now := time.Now().UTC().Truncate(time.Second)
	if l.readTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.readTimeout)
		defer cancel()
	}
	l.calls.Store(0)
	defer func() { l.lastCalls.Store(l.calls.Load()) }()
	var objs []*unstructured.Unstructured
	var scopes []mapper.Scope
	status := mapper.StatusOK
	for _, sp := range l.specs {
		got, err := l.list(ctx, sp)
		if err != nil {
			status = classify(err)
			break
		}
		objs = append(objs, got...)
		if !sp.NoScope {
			ns := sp.Namespace
			if ns == "" {
				ns = "*"
			}
			scopes = append(scopes, mapper.Scope{Kind: sp.Kind, Namespace: ns})
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if status != mapper.StatusOK {
		if l.last == nil {
			return &mapper.Snapshot{Cluster: l.name, Endpoint: l.endpoint, Status: status}
		}
		s := *l.last // faits et portee du dernier succes, horodatages conserves
		s.Status = status
		return &s
	}
	s := &mapper.Snapshot{Cluster: l.name, Endpoint: l.endpoint, Status: status,
		LastSync: now, ObservedAt: now, Scopes: scopes, Objects: objs}
	if l.witness != nil {
		s.WitnessLeaseName = l.witness.Name()
		s.LastWitnessRenew = l.witness.LastSeen()
		s.FreshnessHorizon = l.witness.Theta().Seconds()
	}
	l.last = s
	return s
}

// Cached : le dernier instantane reussi, SANS aucun appel reseau, avec l'age du
// temoin relu en memoire (le Watch temoin vit hors des cycles). nil si jamais lu.
// Sert au rejugement par echeance (A3).
func (l *Live) Cached() *mapper.Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last == nil {
		return nil
	}
	s := *l.last
	if l.witness != nil {
		s.LastWitnessRenew = l.witness.LastSeen()
	}
	return &s
}

func (l *Live) list(ctx context.Context, sp Spec) ([]*unstructured.Unstructured, error) {
	l.calls.Add(1)
	gvr := schema.GroupVersionResource{Group: sp.Group, Version: sp.Version, Resource: sp.Resource}
	ns := sp.Namespace
	if ns == "*" {
		ns = ""
	}
	if sp.MetadataOnly {
		// METADONNEES SEULEMENT : le serveur ne renvoie aucun champ data.
		var lst *metav1.PartialObjectMetadataList
		var err error
		if ns == "" {
			lst, err = l.meta.Resource(gvr).List(ctx, metav1.ListOptions{})
		} else {
			lst, err = l.meta.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
		}
		if err != nil {
			return nil, err
		}
		out := make([]*unstructured.Unstructured, 0, len(lst.Items))
		for i := range lst.Items {
			m := lst.Items[i].ObjectMeta
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "meta.k8s.io/v1", "kind": "PartialObjectMetadata",
				"metadata": map[string]any{"x-sylvatwin-kind": sp.Kind},
			}}
			u.SetName(m.Name)
			u.SetNamespace(m.Namespace)
			u.SetUID(m.UID)
			u.SetLabels(m.Labels)
			u.SetOwnerReferences(m.OwnerReferences)
			out = append(out, u)
		}
		return out, nil
	}
	var lst *unstructured.UnstructuredList
	var err error
	if ns == "" {
		lst, err = l.dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		lst, err = l.dyn.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	out := make([]*unstructured.Unstructured, 0, len(lst.Items))
	for i := range lst.Items {
		o := lst.Items[i]
		if o.GetAPIVersion() == "" {
			o.SetAPIVersion(gvr.GroupVersion().String())
		}
		if o.GetKind() == "" {
			o.SetKind(sp.Kind)
		}
		// managedFields est verbeux : on l'enleve, SAUF pour ConfigMap, dont le
		// greffier a besoin du champ "time" le plus recent (StaleConfigDrift, D4).
		// Jamais de contenu sensible ici : managedFields ne contient aucune donnee,
		// seulement des metadonnees de gestion (manager, operation, time, champs).
		if sp.Kind != "ConfigMap" {
			unstructured.RemoveNestedField(o.Object, "metadata", "managedFields")
		}
		out = append(out, &o)
	}
	return out, nil
}

// classify : forbidden (droits refuses) ou unreachable (tout le reste).
func classify(err error) string {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		return mapper.StatusForbidden
	}
	var se *apierrors.StatusError
	if errors.As(err, &se) && se.ErrStatus.Code == 403 {
		return mapper.StatusForbidden
	}
	return mapper.StatusUnreachable
}
