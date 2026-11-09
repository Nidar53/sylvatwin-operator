package iri

import "testing"

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
