package watcher

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	mchandler "sigs.k8s.io/multicluster-runtime/pkg/handler"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

// recordingHandler : enveloppe un handler et note, pour le journal (A7/correction
// 5), QUEL objet a provoque l'evenement : cluster, type, espace de noms, nom,
// operation, et pour une mise a jour les NOMS des champs qui different. Jamais
// de valeur : aucun contenu (Secret compris) n'est lu ni ecrit ; un Secret n'est
// d'ailleurs observe qu'en metadonnees.
type recordingHandler struct {
	inner   handler.TypedEventHandler[client.Object, mcreconcile.Request]
	cluster multicluster.ClusterName
	rec     func(desc string)
}

func wrapRecording(base mchandler.TypedEventHandlerFunc[client.Object, mcreconcile.Request], rec func(string)) mchandler.TypedEventHandlerFunc[client.Object, mcreconcile.Request] {
	return func(cn multicluster.ClusterName, cl cluster.Cluster) handler.TypedEventHandler[client.Object, mcreconcile.Request] {
		return &recordingHandler{inner: base(cn, cl), cluster: cn, rec: rec}
	}
}

func (h *recordingHandler) Create(ctx context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[mcreconcile.Request]) {
	h.rec(describe(h.cluster, e.Object, "create", nil))
	h.inner.Create(ctx, e, q)
}
func (h *recordingHandler) Update(ctx context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[mcreconcile.Request]) {
	h.rec(describe(h.cluster, e.ObjectNew, "update", changedFields(e.ObjectOld, e.ObjectNew)))
	h.inner.Update(ctx, e, q)
}
func (h *recordingHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[mcreconcile.Request]) {
	h.rec(describe(h.cluster, e.Object, "delete", nil))
	h.inner.Delete(ctx, e, q)
}
func (h *recordingHandler) Generic(ctx context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[mcreconcile.Request]) {
	h.rec(describe(h.cluster, e.Object, "generic", nil))
	h.inner.Generic(ctx, e, q)
}

func describe(cn multicluster.ClusterName, o client.Object, op string, changed []string) string {
	if o == nil {
		return string(cn) + " ? " + op
	}
	kind := o.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		kind = fmt.Sprintf("%T", o)
	}
	name := o.GetName()
	if ns := o.GetNamespace(); ns != "" {
		name = ns + "/" + name
	}
	c := string(cn)
	if c == "" {
		c = "management"
	}
	s := fmt.Sprintf("%s %s %s %s", c, kind, name, op)
	if len(changed) > 0 {
		s += " [" + strings.Join(changed, ",") + "]"
	}
	return s
}

func toMap(o client.Object) map[string]any {
	if u, ok := o.(runtime.Unstructured); ok {
		return u.UnstructuredContent()
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil
	}
	return m
}

// changedFields : noms (pas de valeurs) des champs de premier niveau qui
// different, et des sous-champs de metadata, hors resourceVersion/managedFields.
func changedFields(a, b client.Object) []string {
	if a == nil || b == nil {
		return nil
	}
	ma, mb := toMap(a), toMap(b)
	var out []string
	keys := map[string]bool{}
	for k := range ma {
		keys[k] = true
	}
	for k := range mb {
		keys[k] = true
	}
	for k := range keys {
		if k == "metadata" {
			continue
		}
		if !reflect.DeepEqual(ma[k], mb[k]) {
			out = append(out, k)
		}
	}
	mda, _ := ma["metadata"].(map[string]any)
	mdb, _ := mb["metadata"].(map[string]any)
	sub := map[string]bool{}
	for k := range mda {
		sub[k] = true
	}
	for k := range mdb {
		sub[k] = true
	}
	for k := range sub {
		if k == "resourceVersion" || k == "managedFields" {
			continue
		}
		if !reflect.DeepEqual(mda[k], mdb[k]) {
			out = append(out, "metadata."+k)
		}
	}
	if len(out) == 0 {
		out = append(out, "resourceVersion-seul")
	}
	sort.Strings(out)
	return out
}
