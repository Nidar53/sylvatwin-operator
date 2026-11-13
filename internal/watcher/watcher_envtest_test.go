/*
Tests envtest du controleur continu (partie 1.4 de la mission).

Deux environnements envtest SEPARES simulent H0 :
  - mgmtEnv : le cluster de management (local, lu directement par le Watcher) ;
    les CRD CAPI/Flux reels (exportes en lecture seule depuis le vrai cluster
    de management, cf. internal/watcher/testdata/crds/) y sont installes pour
    que le provider cluster-api puisse decouvrir un objet Cluster ;
  - wlEnv : un cluster "workload" independant, decrit par un Secret
    <cluster>-kubeconfig dans mgmtEnv (convention CAPI), QUE LE WATCHER
    DECOUVRE PAR LE PROVIDER — jamais reference en dur.

Barrieres testees :
 1. Un changement (creation d'un Pod) declenche un cycle complet (mapper+juge)
    en moins de N secondes, SANS boucle de polling (uniquement Watch).
 2. Un cluster workload qui devient injoignable (kubeconfig pointant vers un
    port ferme) produit observationStatus=unreachable, CONSERVE les faits du
    dernier succes (garantie P3), et ne fait pas s'arreter le controleur.

Une instance Oxigraph jetable (meme mecanisme que internal/judge/conformity_test.go)
sert de moteur de regles : sylvatwin-poc/rules est requis (SYLVATWIN_POC),
sinon le test est ignore (meme convention que le reste du depot).
*/
package watcher

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/orange/sylvatwin-operator/internal/judge"
	"github.com/orange/sylvatwin-operator/internal/twin"
)

// startOxigraph : copie du mecanisme de internal/judge/conformity_test.go
// (instance jetable, 127.0.0.1 seulement, base en memoire temporaire).
func startOxigraphForWatcher(t *testing.T, bin string) (*judge.Endpoint, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	dir := t.TempDir()
	cmd := exec.Command(bin, "serve", "--location", dir, "--bind", fmt.Sprintf("127.0.0.1:%d", port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	e, err := judge.NewEndpoint(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if _, err := e.Select(ctx, "ASK {}"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return e, func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }
}

// kubeconfigBytes : serialise un *rest.Config envtest en kubeconfig YAML, au
// format que lit restConfigFromSecret (internal/source/capi.go), avec un
// utilisateur authentifie par certificat client (comme le ferait CAPI).
func kubeconfigBytes(t *testing.T, cfg *rest.Config) []byte {
	t.Helper()
	const ctxName = "default"
	c := clientcmdapi.NewConfig()
	c.Clusters[ctxName] = &clientcmdapi.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: cfg.CAData,
	}
	c.AuthInfos[ctxName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
	}
	c.Contexts[ctxName] = &clientcmdapi.Context{Cluster: ctxName, AuthInfo: ctxName}
	c.CurrentContext = ctxName
	b, err := clientcmd.Write(*c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// requireSylvatwinPocRules : meme convention que le reste du depot (SYLVATWIN_POC).
func requireSylvatwinPocRules(t *testing.T) string {
	t.Helper()
	poc := os.Getenv("SYLVATWIN_POC")
	if poc == "" {
		poc = filepath.Join("..", "..", "..", "sylvatwin-poc")
	}
	if _, err := os.Stat(filepath.Join(poc, "rules", "verdict", "seconds.sparql")); err != nil {
		t.Skipf("sylvatwin-poc introuvable (%s)", poc)
	}
	return filepath.Join(poc, "rules")
}

// setupRBAC : la portee minimale d'un SA "lecteur" impersonne par le Watcher,
// reproduisant sylvatwin-reader (meme principe que sur le vrai banc : Nodes,
// Pods, Deployments, ReplicaSets en lecture, cluster-wide dans ce test pour
// rester simple — le test RBAC namespace-only existe deja par ailleurs via
// l'essai reel documente dans HANDOFF).
func setupRBAC(ctx context.Context, cs *kubernetes.Clientset, readerUser string) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "sylvatwin-reader-test"},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"", "apps"},
			Resources: []string{"nodes", "pods", "namespaces", "configmaps", "deployments", "replicasets", "secrets"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
	if _, err := cs.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		return err
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "sylvatwin-reader-test"},
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: readerUser}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "sylvatwin-reader-test"},
	}
	_, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	return err
}

// TestControleurContinu_DeclenchementParEvenement : barriere 1.4.a. Un Pod
// cree dans le cluster workload doit provoquer un cycle complet en moins de
// 10s, par le seul mecanisme Watch (aucun polling).
func TestControleurContinu_DeclenchementParEvenement(t *testing.T) {
	rulesDir := requireSylvatwinPocRules(t)
	oxBin := os.Getenv("OXIGRAPH_BIN")
	if oxBin == "" {
		t.Skip("OXIGRAPH_BIN non defini")
	}

	mgmtEnv := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("testdata", "crds")}}
	mgmtCfg, err := mgmtEnv.Start()
	if err != nil {
		t.Fatalf("mgmtEnv.Start: %v", err)
	}
	defer mgmtEnv.Stop()

	wlEnv := &envtest.Environment{}
	wlCfg, err := wlEnv.Start()
	if err != nil {
		t.Fatalf("wlEnv.Start: %v", err)
	}
	defer wlEnv.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgmtCS, err := kubernetes.NewForConfig(mgmtCfg)
	if err != nil {
		t.Fatal(err)
	}
	wlCS, err := kubernetes.NewForConfig(wlCfg)
	if err != nil {
		t.Fatal(err)
	}
	readerUser := "system:serviceaccount:sylvatwin-system:sylvatwin-reader"
	if err := setupRBAC(ctx, wlCS, readerUser); err != nil {
		t.Fatalf("RBAC workload: %v", err)
	}

	// Cluster CAPI v1beta2 dans le management, ControlPlaneInitialized=true,
	// endpoint DIFFERENT du management (sinon le provider le considere comme
	// le management lui-meme et ne l'engage pas).
	const (
		wlNS   = "my-workload-envtest"
		wlName = "my-workload-envtest"
	)
	if _, err := mgmtCS.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: wlNS}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("namespace workload: %v", err)
	}
	mgmtDyn := dynFor(t, mgmtCfg)
	host, port := splitHostPort(t, wlCfg.Host)
	cl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": wlName, "namespace": wlNS},
		"spec": map[string]any{
			"controlPlaneEndpoint": map[string]any{"host": host, "port": port},
		},
	}}
	gvrCluster := schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"}
	created, err := mgmtDyn.Resource(gvrCluster).Namespace(wlNS).Create(ctx, cl, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create Cluster CAPI: %v", err)
	}
	trueVal := true
	_ = unstructured.SetNestedField(created.Object, true, "status", "initialization", "controlPlaneInitialized")
	_, err = mgmtDyn.Resource(gvrCluster).Namespace(wlNS).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("update status Cluster CAPI: %v", err)
	}
	_ = trueVal

	// Secret <cluster>-kubeconfig, convention CAPI (cle "value"), JAMAIS lu
	// par le test autrement que via ce chemin (meme garde que source/capi.go).
	kcBytes := kubeconfigBytes(t, wlCfg)
	_, err = mgmtCS.CoreV1().Secrets(wlNS).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: wlName + "-kubeconfig"},
		Data:       map[string][]byte{"value": kcBytes},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create Secret kubeconfig: %v", err)
	}

	oxE, stopOx := startOxigraphForWatcher(t, oxBin)
	defer stopOx()

	outDir := t.TempDir()
	var gotCycle chan struct{} = make(chan struct{}, 10)
	var lastErr error
	opts := Options{
		MetricsBindAddress: "0",
		ManagementConfig:   mgmtCfg,
		ManagementName:     "management-cluster",
		NFNamespace:        "sylvatwin-5gc",
		CacheDir:           t.TempDir(),
		OutDir:             outDir,
		JudgePipeline:      twin.Twin{Judge: &judge.Pipeline{E: oxE, RulesDir: rulesDir, Validity: 600}},
		Debounce:           500 * time.Millisecond,
		OnReport: func(trigger string, rep *twin.CycleReport, err error) {
			lastErr = err
			gotCycle <- struct{}{}
		},
	}
	w, err := New(ctx, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = w.Start(ctx) }()

	// Attendre que le cluster workload soit engage (le provider doit d'abord
	// reconcilier le Cluster CAPI) avant d'injecter l'evenement.
	deadline := time.Now().Add(20 * time.Second)
	for len(w.EngagedClusters()) == 0 && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if len(w.EngagedClusters()) == 0 {
		t.Fatal("cluster workload jamais engage par le provider CAPI")
	}
	// Vider les cycles eventuels declenches par l'engagement lui-meme.
	drain(gotCycle, 500*time.Millisecond)

	// --- L'EVENEMENT : creation d'un Pod dans le workload. ---
	t0 := time.Now()
	if _, err := wlCS.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "sylvatwin-5gc"}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("namespace NF: %v", err)
	}
	if _, err := wlCS.CoreV1().Pods("sylvatwin-5gc").Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "declencheur"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create Pod: %v", err)
	}

	select {
	case <-gotCycle:
		elapsed := time.Since(t0)
		t.Logf("cycle declenche en %s apres l'evenement", elapsed)
		if elapsed > 10*time.Second {
			t.Errorf("declenchement trop lent : %s (attendu < 10s)", elapsed)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("aucun cycle declenche dans les 15s suivant l'evenement (Watch inoperant)")
	}
	if lastErr != nil {
		t.Fatalf("cycle en erreur : %v", lastErr)
	}
}

// TestControleurContinu_ClusterInjoignable : barriere 1.4.b. Un workload
// injoignable (kubeconfig pointant vers un port ferme APRES le premier cycle
// reussi) doit produire observationStatus=unreachable, conserver les faits du
// dernier succes (P3), et ne jamais faire planter le Watcher.
func TestControleurContinu_ClusterInjoignable(t *testing.T) {
	rulesDir := requireSylvatwinPocRules(t)
	oxBin := os.Getenv("OXIGRAPH_BIN")
	if oxBin == "" {
		t.Skip("OXIGRAPH_BIN non defini")
	}

	mgmtEnv := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("testdata", "crds")}}
	mgmtCfg, err := mgmtEnv.Start()
	if err != nil {
		t.Fatalf("mgmtEnv.Start: %v", err)
	}
	defer mgmtEnv.Stop()

	wlEnv := &envtest.Environment{}
	wlCfg, err := wlEnv.Start()
	if err != nil {
		t.Fatalf("wlEnv.Start: %v", err)
	}
	// wlEnv.Stop() est appele EXPLICITEMENT plus bas (au milieu du test),
	// pas en defer : c'est justement l'evenement "cluster injoignable".

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgmtCS, err := kubernetes.NewForConfig(mgmtCfg)
	if err != nil {
		t.Fatal(err)
	}
	wlCS, err := kubernetes.NewForConfig(wlCfg)
	if err != nil {
		t.Fatal(err)
	}
	readerUser := "system:serviceaccount:sylvatwin-system:sylvatwin-reader"
	if err := setupRBAC(ctx, wlCS, readerUser); err != nil {
		t.Fatalf("RBAC workload: %v", err)
	}

	const (
		wlNS   = "my-workload-envtest2"
		wlName = "my-workload-envtest2"
	)
	if _, err := mgmtCS.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: wlNS}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("namespace workload: %v", err)
	}
	mgmtDyn := dynFor(t, mgmtCfg)
	host, port := splitHostPort(t, wlCfg.Host)
	cl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": wlName, "namespace": wlNS},
		"spec": map[string]any{
			"controlPlaneEndpoint": map[string]any{"host": host, "port": port},
		},
	}}
	gvrCluster := schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"}
	created, err := mgmtDyn.Resource(gvrCluster).Namespace(wlNS).Create(ctx, cl, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create Cluster CAPI: %v", err)
	}
	_ = unstructured.SetNestedField(created.Object, true, "status", "initialization", "controlPlaneInitialized")
	if _, err = mgmtDyn.Resource(gvrCluster).Namespace(wlNS).UpdateStatus(ctx, created, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update status Cluster CAPI: %v", err)
	}

	kcBytes := kubeconfigBytes(t, wlCfg)
	if _, err = mgmtCS.CoreV1().Secrets(wlNS).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: wlName + "-kubeconfig"},
		Data:       map[string][]byte{"value": kcBytes},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create Secret kubeconfig: %v", err)
	}
	// Namespace + 1 Pod pour avoir des faits non vides au premier succes.
	if _, err := wlCS.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "sylvatwin-5gc"}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("namespace NF: %v", err)
	}
	if _, err := wlCS.CoreV1().Pods("sylvatwin-5gc").Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "temoin-p3"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create Pod: %v", err)
	}

	oxE, stopOx := startOxigraphForWatcher(t, oxBin)
	defer stopOx()

	outDir := t.TempDir()
	reports := make(chan *twin.CycleReport, 20)
	opts := Options{
		MetricsBindAddress: "0",
		ManagementConfig:   mgmtCfg,
		ManagementName:     "management-cluster",
		NFNamespace:        "sylvatwin-5gc",
		CacheDir:           t.TempDir(),
		OutDir:             outDir,
		JudgePipeline:      twin.Twin{Judge: &judge.Pipeline{E: oxE, RulesDir: rulesDir, Validity: 600}},
		Debounce:           300 * time.Millisecond,
		OnReport: func(trigger string, rep *twin.CycleReport, err error) {
			if err == nil {
				reports <- rep
			}
		},
	}
	w, err := New(ctx, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = w.Start(ctx) }()

	deadline := time.Now().Add(20 * time.Second)
	for len(w.EngagedClusters()) == 0 && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if len(w.EngagedClusters()) == 0 {
		t.Fatal("cluster workload jamais engage")
	}

	// Attendre un premier cycle ou le workload est "ok" avec des faits.
	var okRep *twin.CycleReport
	waitDeadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(waitDeadline) {
		select {
		case rep := <-reports:
			st, ok := rep.Clusters[string(w.EngagedClusters()[0])]
			if ok && st == "ok" && rep.Objects[string(w.EngagedClusters()[0])] > 0 {
				okRep = rep
			}
		case <-time.After(500 * time.Millisecond):
		}
		if okRep != nil {
			break
		}
	}
	if okRep == nil {
		t.Fatal("jamais observe le workload 'ok' avec des objets avant la coupure")
	}
	factsBeforeCut := okRep.Objects[string(w.EngagedClusters()[0])]
	t.Logf("avant coupure : %d objets observes sur le workload", factsBeforeCut)

	// --- L'EVENEMENT : le workload devient injoignable. ---
	if err := wlEnv.Stop(); err != nil {
		t.Logf("wlEnv.Stop (attendu, cluster injoignable) : %v", err)
	}

	// Attendre un cycle ulterieur (force par le resync, puisqu'un cluster mort
	// ne peut plus emettre d'evenement Watch) qui voit "unreachable".
	var unreachableRep *twin.CycleReport
	waitDeadline = time.Now().Add(20 * time.Second)
drainLoop:
	for time.Now().Before(waitDeadline) {
		select {
		case rep := <-reports:
			st, ok := rep.Clusters[string(w.EngagedClusters()[0])]
			if ok && st == "unreachable" {
				unreachableRep = rep
				break drainLoop
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
	if unreachableRep == nil {
		t.Fatal("jamais observe observationStatus=unreachable apres la coupure")
	}
	// P3 : les faits du dernier succes doivent etre CONSERVES (pas remis a 0).
	gotObjects := unreachableRep.Objects[string(w.EngagedClusters()[0])]
	if gotObjects != factsBeforeCut {
		t.Errorf("garantie P3 violee : objets avant coupure=%d, objets apres coupure (unreachable)=%d (attendu : identiques)",
			factsBeforeCut, gotObjects)
	}
	t.Logf("apres coupure : status=unreachable, objets conserves=%d (P3 respectee)", gotObjects)
}

// --- utilitaires de test ---

func drain(ch chan struct{}, d time.Duration) {
	deadline := time.After(d)
	for {
		select {
		case <-ch:
		case <-deadline:
			return
		}
	}
}

func dynFor(t *testing.T, cfg *rest.Config) dynamic.Interface {
	t.Helper()
	d, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// splitHostPort : host et port numerique a partir de "https://127.0.0.1:12345".
func splitHostPort(t *testing.T, rawHost string) (string, int64) {
	t.Helper()
	s := rawHost
	for _, p := range []string{"https://", "http://"} {
		if len(s) > len(p) && s[:len(p)] == p {
			s = s[len(p):]
			break
		}
	}
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		t.Fatal(err)
	}
	var port int64
	if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
		t.Fatal(err)
	}
	return h, port
}
