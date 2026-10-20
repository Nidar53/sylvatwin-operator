/*
twinsync_controller.go — LA BOUCLE DE RECONCILIATION.

Difference fondamentale avec fact_writer.py v1 :

	v1 : chaque evenement Watch ecrit DIRECTEMENT dans le graphe (mutation
	     incrementale), d'ou set_single() / set_multi() / remove_individual()
	     pour garantir l'idempotence a la main.

	v2 : chaque evenement met simplement le TwinSync en file. La reconciliation
	     RECONSTRUIT le graphe entier depuis le cache de l'informer.
	     -> plus aucune machinerie d'idempotence : elle est structurelle.
	     -> 20 evenements dans une rafale = 1 seule reconciliation (workqueue).
	     -> resync periodique : une desynchronisation silencieuse se repare seule.

L'invariant reste intact : aucune comparaison desired/actual, aucune classe
Drift. La detection vit dans detect.py (SPARQL).
*/
package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sylvatwinv1alpha1 "github.com/orange/sylvatwin-operator/api/v1alpha1"
	"github.com/orange/sylvatwin-operator/internal/desired"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

type TwinSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// --- RBAC : lecture seule sur le cluster observe. -------------------------
// Le jumeau ne doit JAMAIS pouvoir modifier le cluster. Seuls get/list/watch.
// Les droits en ecriture se limitent a la CRD TwinSync elle-meme (status).
//
// +kubebuilder:rbac:groups=sylvatwin.orange.com,resources=twinsyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sylvatwin.orange.com,resources=twinsyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sylvatwin.orange.com,resources=twinsyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

func (r *TwinSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ts sylvatwinv1alpha1.TwinSync
	if err := r.Get(ctx, req.NamespacedName, &ts); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	resync := 30 * time.Second
	if ts.Spec.ResyncPeriod.Duration > 0 {
		resync = ts.Spec.ResyncPeriod.Duration
	}

	graph, counts, err := r.buildGraph(ctx, &ts)
	if err != nil {
		r.setFailed(ctx, &ts, err)
		return ctrl.Result{RequeueAfter: resync}, err
	}

	if err := r.writeGraph(&ts, graph); err != nil {
		r.setFailed(ctx, &ts, err)
		return ctrl.Result{RequeueAfter: resync}, err
	}

	logger.Info("jumeau synchronise",
		"triplets", graph.Len(), "pods", counts.pods,
		"nodes", counts.nodes, "deployments", counts.deployments)

	now := metav1.Now()
	ts.Status.Phase = "Synced"
	ts.Status.LastSyncTime = &now // <- le heartbeat
	ts.Status.Triples = graph.Len()
	ts.Status.ObservedPods = counts.pods
	ts.Status.ObservedNodes = counts.nodes
	ts.Status.ObservedDeployments = counts.deployments
	ts.Status.Message = fmt.Sprintf("graphe ecrit dans %s", r.outputPath(&ts))
	if err := r.Status().Update(ctx, &ts); err != nil && !apierrors.IsConflict(err) {
		return ctrl.Result{}, err
	}

	// Requeue systematique : c'est le resync periodique, le filet de securite
	// contre une desynchronisation silencieuse (evenement perdu, watch mort).
	return ctrl.Result{RequeueAfter: resync}, nil
}

type observedCounts struct{ pods, nodes, deployments int }

// buildGraph reconstruit le graphe ENTIER depuis le cache de l'informer.
// Aucune mutation incrementale : le resultat ne depend que de l'etat courant
// du cluster et de Git. C'est cela qui rend la reconciliation idempotente
// par construction, sans set_single().
func (r *TwinSyncReconciler) buildGraph(
	ctx context.Context, ts *sylvatwinv1alpha1.TwinSync,
) (*rdf.Graph, observedCounts, error) {

	g := rdf.New()
	var c observedCounts
	now := time.Now()

	// ---- COTE DESIRED (Git) — relu a CHAQUE reconciliation ---------------
	deps, err := desired.Read(ts.Spec.GitDir)
	if err != nil {
		return nil, c, fmt.Errorf("lecture du cote desired : %w", err)
	}
	for _, d := range deps {
		mapDesiredDeployment(g, d)
	}

	listOpts := r.listOptions(ts)

	// ---- COTE ACTUAL : Deployments ---------------------------------------
	var depList appsv1.DeploymentList
	if err := r.List(ctx, &depList, listOpts...); err != nil {
		return nil, c, fmt.Errorf("list deployments : %w", err)
	}
	for i := range depList.Items {
		mapActualDeployment(g, &depList.Items[i], now)
	}
	c.deployments = len(depList.Items)

	// ---- COTE ACTUAL : Pods ----------------------------------------------
	var podList corev1.PodList
	if err := r.List(ctx, &podList, listOpts...); err != nil {
		return nil, c, fmt.Errorf("list pods : %w", err)
	}
	for i := range podList.Items {
		pod := &podList.Items[i]
		mapPod(g, pod, r.resolveOwnerDeployment(ctx, pod), now)
	}
	c.pods = len(podList.Items)

	// ---- COTE ACTUAL : Nodes (cluster-scoped, pas de filtre namespace) ---
	var nodeList corev1.NodeList
	if err := r.List(ctx, &nodeList); err != nil {
		return nil, c, fmt.Errorf("list nodes : %w", err)
	}
	for i := range nodeList.Items {
		mapNode(g, &nodeList.Items[i], now)
	}
	c.nodes = len(nodeList.Items)

	return g, c, nil
}

// resolveOwnerDeployment remonte la vraie chaine d'appartenance :
//
//	Pod --ownerRef--> ReplicaSet --ownerRef--> Deployment
//
// C'est la correction du point faible de la v1, qui devinait le Deployment
// depuis le label "app". Le ReplicaSet n'est pas ecrit dans le graphe : la
// TBox v0 ne le connait pas, on le traverse seulement. Chaine vide si echec.
func (r *TwinSyncReconciler) resolveOwnerDeployment(ctx context.Context, pod *corev1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		var rs appsv1.ReplicaSet
		key := types.NamespacedName{Namespace: pod.Namespace, Name: ref.Name}
		if err := r.Get(ctx, key, &rs); err != nil {
			return ""
		}
		for _, rsRef := range rs.OwnerReferences {
			if rsRef.Kind == "Deployment" {
				return rsRef.Name
			}
		}
	}
	return ""
}

func (r *TwinSyncReconciler) listOptions(ts *sylvatwinv1alpha1.TwinSync) []client.ListOption {
	if len(ts.Spec.Namespaces) == 1 {
		return []client.ListOption{client.InNamespace(ts.Spec.Namespaces[0])}
	}
	return nil // tous les namespaces
}

func (r *TwinSyncReconciler) outputPath(ts *sylvatwinv1alpha1.TwinSync) string {
	if ts.Spec.OutputPath != "" {
		return ts.Spec.OutputPath
	}
	return "sylvatwin-graph.ttl"
}

// writeGraph ecrit le fichier de maniere atomique (fichier temporaire + rename).
// Sans cela, detect.py pourrait lire un .ttl tronque en pleine ecriture et
// echouer de facon intermittente — un bug tres penible a diagnostiquer.
func (r *TwinSyncReconciler) writeGraph(ts *sylvatwinv1alpha1.TwinSync, g *rdf.Graph) error {
	var tbox []byte
	if ts.Spec.TBoxPath != "" {
		b, err := os.ReadFile(ts.Spec.TBoxPath)
		if err == nil {
			tbox = b
		}
	}

	out := r.outputPath(ts)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil && filepath.Dir(out) != "." {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(out), ".sylvatwin-*.ttl")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := g.Serialize(tmp, tbox); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
}

func (r *TwinSyncReconciler) setFailed(ctx context.Context, ts *sylvatwinv1alpha1.TwinSync, err error) {
	ts.Status.Phase = "Failed"
	ts.Status.Message = err.Error()
	_ = r.Status().Update(ctx, ts)
}

// SetupWithManager cable les trois sources d'evenements.
//
// Chaque Pod / Node / Deployment qui bouge ne declenche PAS une ecriture :
// il met simplement chaque TwinSync en file. La workqueue dedoublonne, donc
// une rafale de 20 evenements produit une seule reconciliation.
func (r *TwinSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	toTwinSyncs := handler.EnqueueRequestsFromMapFunc(r.mapToTwinSyncs)

	return ctrl.NewControllerManagedBy(mgr).
		For(&sylvatwinv1alpha1.TwinSync{}).
		Watches(&corev1.Pod{}, toTwinSyncs).
		Watches(&corev1.Node{}, toTwinSyncs).
		Watches(&appsv1.Deployment{}, toTwinSyncs).
		Named("twinsync").
		Complete(r)
}

func (r *TwinSyncReconciler) mapToTwinSyncs(ctx context.Context, _ client.Object) []reconcile.Request {
	var list sylvatwinv1alpha1.TwinSyncList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, ts := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: ts.Namespace, Name: ts.Name},
		})
	}
	return reqs
}
