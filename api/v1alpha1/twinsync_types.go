/*
SylvaTwin — CRD TwinSync
========================
Decrit UNE instance de jumeau semantique pour le cluster courant.

Invariant de la these : cet operateur n'ecrit QUE des faits. Aucune comparaison
desired/actual, aucune classe Drift. La detection reste dans detect.py (SPARQL).
*/
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TwinSyncSpec : la configuration desiree du jumeau.
type TwinSyncSpec struct {
	// Identifiant logique du cluster observe. Sert de prefixe d'URI en
	// multi-cluster ; laisser vide en mono-cluster pour rester compatible
	// avec les URI produites par fact_writer.py v1.
	// +optional
	ClusterID string `json:"clusterID,omitempty"`

	// Dossier local jouant le role du depot Git (cote desired).
	// Vide => le cote desired n'est pas alimente.
	// +optional
	GitDir string `json:"gitDir,omitempty"`

	// Fichier Turtle de sortie (le jumeau).
	// +kubebuilder:default="sylvatwin-graph.ttl"
	OutputPath string `json:"outputPath,omitempty"`

	// TBox a inclure en tete du fichier de sortie, pour qu'il soit
	// autoportant et ouvrable directement dans Protege.
	// +optional
	TBoxPath string `json:"tboxPath,omitempty"`

	// Reconciliation periodique meme sans evenement. C'est le filet de
	// securite contre une desynchronisation silencieuse.
	// +kubebuilder:default="30s"
	ResyncPeriod metav1.Duration `json:"resyncPeriod,omitempty"`

	// Namespaces observes. Vide => tous.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
}

// TwinSyncStatus : l'etat observe du jumeau, rempli par l'operateur.
type TwinSyncStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`

	// Heartbeat : date de la derniere reconciliation reussie.
	// C'est ce champ qui qualifiera la fraicheur en multi-cluster.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// +optional
	Triples int `json:"triples,omitempty"`
	// +optional
	ObservedPods int `json:"observedPods,omitempty"`
	// +optional
	ObservedNodes int `json:"observedNodes,omitempty"`
	// +optional
	ObservedDeployments int `json:"observedDeployments,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tsync,scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Triples",type=integer,JSONPath=`.status.triples`
// +kubebuilder:printcolumn:name="Pods",type=integer,JSONPath=`.status.observedPods`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.observedNodes`
// +kubebuilder:printcolumn:name="LastSync",type=date,JSONPath=`.status.lastSyncTime`

// TwinSync est le jumeau semantique d'un cluster Kubernetes.
type TwinSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TwinSyncSpec   `json:"spec,omitempty"`
	Status TwinSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type TwinSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TwinSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TwinSync{}, &TwinSyncList{})
		return nil
	})
}
