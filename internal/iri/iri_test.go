package iri

import "testing"

// TestCanonicalCluster : D2. Nom canonique = "namespace/nom", SOUS LA MEME
// FORME que multicluster.ClusterName(types.NamespacedName.String()) du
// provider cluster-api de multicluster-runtime v0.24.1 (provider.go,
// Reconcile : "key := multicluster.ClusterName(req.NamespacedName.String())",
// et NamespacedName.String() = "namespace/nom"). SplitCanonicalCluster doit
// en etre l'inverse exact.
func TestCanonicalCluster(t *testing.T) {
	cases := []struct{ ns, name, want string }{
		{"my-workload-kubeadm-capd", "my-workload-kubeadm-capd", "my-workload-kubeadm-capd/my-workload-kubeadm-capd"},
		{"ns1", "cluster-a", "ns1/cluster-a"},
	}
	for _, c := range cases {
		got := CanonicalCluster(c.ns, c.name)
		if got != c.want {
			t.Errorf("CanonicalCluster(%q,%q) = %q, attendu %q", c.ns, c.name, got, c.want)
		}
		ns, name, ok := SplitCanonicalCluster(got)
		if !ok || ns != c.ns || name != c.name {
			t.Errorf("SplitCanonicalCluster(%q) = (%q,%q,%v), attendu (%q,%q,true)", got, ns, name, ok, c.ns, c.name)
		}
	}
	if _, _, ok := SplitCanonicalCluster("management-cluster"); ok {
		t.Error("nom NU (sans '/') : SplitCanonicalCluster doit renvoyer ok=false")
	}
}

// Les attendus sont recopies de sylvatwin-core-v1-example.ttl (sylvatwin-poc, test).
func TestIRI(t *testing.T) {
	const w = "my-workload-kubeadm-capd"
	cases := []struct{ got, want string }{
		{Cluster(w), "https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd"},
		{Resource(w, "sylvatwin-5gc", "Deployment", "apps", "udm"),
			"https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/sylvatwin-5gc/Deployment/udm"},
		{Resource(w, "", "Node", "", "my-workload-kubeadm-capd-control-plane-6w58l"),
			"https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/_/Node/my-workload-kubeadm-capd-control-plane-6w58l"},
		{Resource("management-cluster", "sylvatwin-flux", "Kustomization", "kustomize.toolkit.fluxcd.io", "sylvatwin-5gc"),
			"https://w3id.org/sylvatwin/res/management-cluster/sylvatwin-flux/Kustomization.kustomize.toolkit.fluxcd.io/sylvatwin-5gc"},
		{LabelOf(Resource(w, "", "Node", "", "n1"), "kubernetes.io/os"),
			"https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/_/Node/n1/label/kubernetes.io%2Fos"},
		{LabelOf("x", "cluster"), "x/label/cluster"},
		{NodeSelectorOf("d", "sylvatwin.orange.com/zone"), "d/nodeSelector/sylvatwin.orange.com%2Fzone"},
		{NetworkFunction(w, "sylvatwin-5gc", "udm"),
			"https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/sylvatwin-5gc/NetworkFunction/udm"},
		{Interface("n", "SBI"), "n/interface/SBI"},
		{PLMN("999", "70"), "https://w3id.org/sylvatwin/res/_/PLMN/999-70"},
		{Slice(1, ""), "https://w3id.org/sylvatwin/res/_/Slice/sst-1"},
		{Scope(w, "Secret", "sylvatwin-5gc"), "https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/_/scope/Secret/sylvatwin-5gc"},
		{Scope(w, "Node", "*"), "https://w3id.org/sylvatwin/res/my-workload-kubeadm-capd/_/scope/Node/_all"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("cas %d : obtenu %q, attendu %q", i, c.got, c.want)
		}
	}
}
