/*
mapper.go — LA PROJECTION Kubernetes -> ontologie.

C'est le coeur du "greffier". Chaque fonction traduit un objet Kubernetes
en triplets. Aucune comparaison, aucun jugement, aucune classe Drift.

Les proprietes ecrites sont exactement celles de la TBox v0 :

	Deployment : desiredReplicas, actualReplicas, requiresNodeLabel, observedAt
	Pod        : partOf, runsOn, observedAt
	Node       : offersLabel, observedAt
*/
package controller

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/orange/sylvatwin-operator/internal/desired"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// labelTerm aplatit une paire cle/valeur en "cle=valeur".
//
// ATTENTION : cette convention est partagee par requiresNodeLabel (cote Git)
// et offersLabel (cote cluster). C'est UNIQUEMENT parce que les deux cotes
// ecrivent la meme chaine que le FILTER NOT EXISTS de Q1 fonctionne.
// Dependance implicite a documenter ; a remplacer par une classe Label
// (hasKey / hasValue) quand la TBox s'etendra.
func labelTerm(k, v string) rdf.Term {
	return rdf.Str(fmt.Sprintf("%s=%s", k, v))
}

// mapDesiredDeployment ecrit le cote DESIRED (source : Git).
func mapDesiredDeployment(g *rdf.Graph, d desired.Deployment) {
	subj := rdf.IRI("", d.Name) // pas de prefixe : point de rencontre avec le cluster

	g.Add(subj, rdf.A, rdf.Class("Deployment"))
	g.Add(subj, rdf.Prop("desiredReplicas"), rdf.Int(d.Replicas))
	for k, v := range d.NodeSelector {
		g.Add(subj, rdf.Prop("requiresNodeLabel"), labelTerm(k, v))
	}
}

// mapActualDeployment ecrit le cote ACTUAL (source : cluster).
// Note l'URI : rdf.IRI("", name) — la MEME que ci-dessus. C'est ce qui fait
// atterrir desiredReplicas et actualReplicas sur un individu unique (Option C).
func mapActualDeployment(g *rdf.Graph, dep *appsv1.Deployment, now time.Time) {
	subj := rdf.IRI("", dep.Name)

	g.Add(subj, rdf.A, rdf.Class("Deployment"))
	g.Add(subj, rdf.Prop("actualReplicas"), rdf.Int(int(dep.Status.Replicas)))
	g.Add(subj, rdf.Prop("observedAt"), rdf.DateTime(now))
}

// mapPod ecrit les deux relations structurelles du Pod.
//
// ownerName : nom du Deployment resolu via ownerReferences (Pod -> ReplicaSet
// -> Deployment). Chaine vide si non resolu : on retombe alors sur le label
// "app", comme en v1.
func mapPod(g *rdf.Graph, pod *corev1.Pod, ownerName string, now time.Time) {
	subj := rdf.IRI("pod", pod.Name)

	g.Add(subj, rdf.A, rdf.Class("Pod"))

	if ownerName == "" {
		ownerName = pod.Labels["app"] // repli v1
	}
	if ownerName != "" {
		g.Add(subj, rdf.Prop("partOf"), rdf.IRI("", ownerName))
	}

	// Garde volontaire : un Pod Pending n'a pas de nodeName, il entre donc
	// dans le graphe SANS runsOn. Q1 exige ce triplet, donc un pod non
	// planifie n'apparait jamais comme derive de placement — ce qui est juste :
	// il n'est pas mal place, il n'est pas place.
	if pod.Spec.NodeName != "" {
		g.Add(subj, rdf.Prop("runsOn"), rdf.IRI("node", pod.Spec.NodeName))
	}

	g.Add(subj, rdf.Prop("observedAt"), rdf.DateTime(now))
}

// mapNode ecrit TOUS les labels du noeud, sans filtrage.
// Le greffier ne decide pas quel fait est pertinent : filtrer, c'est deja juger.
func mapNode(g *rdf.Graph, node *corev1.Node, now time.Time) {
	subj := rdf.IRI("node", node.Name)

	g.Add(subj, rdf.A, rdf.Class("Node"))
	for k, v := range node.Labels {
		g.Add(subj, rdf.Prop("offersLabel"), labelTerm(k, v))
	}
	g.Add(subj, rdf.Prop("observedAt"), rdf.DateTime(now))
}
