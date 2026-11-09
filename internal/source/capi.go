package source

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Reader : identite en lecture seule dans les workloads (impersonation).
const (
	ReaderNamespace = "sylvatwin-system"
	ReaderName      = "sylvatwin-reader"
)

// ReaderUser : nom d'utilisateur Kubernetes du compte de lecture.
func ReaderUser() string {
	return "system:serviceaccount:" + ReaderNamespace + ":" + ReaderName
}

// Discovered : un cluster decrit par un objet Cluster de Cluster API.
type Discovered struct {
	Name      string // nom du Cluster CAPI
	Namespace string
	Endpoint  string // host:port de spec.controlPlaneEndpoint
	Ready     bool   // status.initialization.controlPlaneInitialized
}

var gvrCluster = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"}

// DiscoverCAPI : liste les Cluster CAPI du management (lecture seule).
// Equivalent du provider cluster-api de multicluster-runtime v0.24.1 (meme
// critere de disponibilite : controlPlaneInitialized), en version "un coup"
// adaptee au mode --once ; le controleur continu utilise le provider.
func DiscoverCAPI(ctx context.Context, mgmt *rest.Config) ([]Discovered, error) {
	dyn, err := dynamic.NewForConfig(mgmt)
	if err != nil {
		return nil, err
	}
	lst, err := dyn.Resource(gvrCluster).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []Discovered
	for _, c := range lst.Items {
		host, _, _ := unstructured.NestedString(c.Object, "spec", "controlPlaneEndpoint", "host")
		port, _, _ := unstructured.NestedInt64(c.Object, "spec", "controlPlaneEndpoint", "port")
		if port == 0 {
			port = 6443
		}
		ready, _, _ := unstructured.NestedBool(c.Object, "status", "initialization", "controlPlaneInitialized")
		d := Discovered{Name: c.GetName(), Namespace: c.GetNamespace(), Ready: ready}
		if host != "" {
			d.Endpoint = fmt.Sprintf("%s:%d", host, port)
		}
		out = append(out, d)
	}
	return out, nil
}

// WorkloadConfig : rest.Config d'un workload a partir du Secret kubeconfig de
// CAPI (<cluster>-kubeconfig, cle "value"), EN MEMOIRE SEULEMENT, avec
// impersonation obligatoire du compte de lecture.
//
// Le contenu du Secret n'est ni journalise ni renvoye : seul le rest.Config sort
// de cette fonction, et il est toujours impersonne.
func WorkloadConfig(ctx context.Context, mgmt *rest.Config, d Discovered) (*rest.Config, error) {
	cs, err := kubernetes.NewForConfig(mgmt)
	if err != nil {
		return nil, err
	}
	sec, err := cs.CoreV1().Secrets(d.Namespace).Get(ctx, d.Name+"-kubeconfig", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Secret kubeconfig de %s/%s : %w", d.Namespace, d.Name, redact(err))
	}
	cfg, err := restConfigFromSecret(sec)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig de %s/%s illisible", d.Namespace, d.Name) // pas de detail : pourrait contenir des octets du secret
	}
	cfg.Impersonate = rest.ImpersonationConfig{UserName: ReaderUser()}
	return cfg, nil
}

func restConfigFromSecret(sec *corev1.Secret) (*rest.Config, error) {
	raw := sec.Data["value"]
	if len(raw) == 0 {
		raw = sec.Data["value.yaml"]
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("cle value absente")
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	// effacer la copie brute au plus tot (le rest.Config garde ce qu'il faut)
	for i := range raw {
		raw[i] = 0
	}
	return cfg, err
}

// redact : une erreur d'API n'expose jamais le contenu d'un Secret, mais on
// borne quand meme le message.
func redact(err error) error {
	s := err.Error()
	if len(s) > 200 {
		s = s[:200]
	}
	return fmt.Errorf("%s", strings.ReplaceAll(s, "\n", " "))
}
