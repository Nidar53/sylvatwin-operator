package mapper

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/orange/sylvatwin-operator/internal/iri"
	"github.com/orange/sylvatwin-operator/internal/rdf"
)

// NFLabel : label pose dans Git sur chaque Deployment de NF (realizedBy).
const NFLabel = "sylvatwin.orange.com/nf"

// Classe 3GPP de l'ontologie pour chaque valeur du label.
var nfClass = map[string]string{
	"amf": "AMF", "smf": "SMF", "upf": "UPF", "nrf": "NRF", "ausf": "AUSF",
	"udm": "UDM", "udr": "UDR", "pcf": "PCF", "nssf": "NSSF", "bsf": "BSF",
}

// Ports par defaut d'Open5GS v2.7.2 quand la configuration ne les donne pas
// (exemple de reference : N2 38412/SCTP, N4 8805/UDP).
var defaultPorts = map[string]struct {
	port  int64
	proto string
}{"ngap": {38412, "SCTP"}, "pfcp": {8805, "UDP"}, "gtpu": {2152, "UDP"}}

var ifaceName = map[string]string{"ngap": "N2", "pfcp": "N4", "gtpu": "N3"}

// mapL4 : adaptateur Open5GS. Une NF par Deployment portant le label NFLabel ;
// configuration lue dans la ConfigMap montee (cle <nf>.yaml).
//
// Le greffier N'INVENTE PAS : une NF qui ne declare pas de PLMN n'a pas de
// :hasPlmn (cas de l'UDM) ; une dependance n'est ecrite que si l'URI du NRF
// designe un Service d'une NF connue.
func mapL4(g *rdf.Graph, s *Snapshot) {
	cms := map[string]*unstructured.Unstructured{}
	byName := map[string]string{} // nom de Deployment -> valeur du label nf
	for _, o := range s.Objects {
		switch kindOf(o) {
		case "ConfigMap":
			cms[o.GetNamespace()+"/"+o.GetName()] = o
		case "Deployment":
			if nf := o.GetLabels()[NFLabel]; nf != "" {
				byName[o.GetNamespace()+"/"+o.GetName()] = nf
			}
		}
	}
	for _, o := range s.Objects {
		if kindOf(o) != "Deployment" {
			continue
		}
		nf := o.GetLabels()[NFLabel]
		class, ok := nfClass[nf]
		if !ok {
			continue
		}
		ns := o.GetNamespace()
		n := rdf.IRI(iri.NetworkFunction(s.Cluster, ns, nf))
		g.Add(n, rdf.Type, rdf.C(class))
		g.Add(n, rdf.C("nfName"), rdf.Str(nf))
		g.Add(n, rdf.C("realizedBy"), resIRI(s.Cluster, o))

		cfg := nfConfig(o, cms, nf)
		if cfg == nil {
			continue
		}
		sect, _ := cfg[nf].(map[string]any)
		if sect == nil {
			continue
		}
		// --- interfaces : SBI (port donne), N2/N4/N3 (port par defaut Open5GS)
		if srv := firstServer(sect, "sbi"); srv != nil {
			port := int64(7777)
			if p, ok := srv["port"].(float64); ok {
				port = int64(p)
			}
			iface(g, n, "SBI", port, "TCP")
		}
		for _, key := range []string{"ngap", "pfcp", "gtpu"} {
			if srv := firstServer(sect, key); srv != nil {
				d := defaultPorts[key]
				port := d.port
				if p, ok := srv["port"].(float64); ok {
					port = int64(p)
				}
				iface(g, n, ifaceName[key], port, d.proto)
			}
		}
		// --- dependances : sbi.client.nrf[].uri (et scp[].uri) -> NF du Service
		for _, cli := range []string{"nrf", "scp"} {
			for _, u := range clientURIs(sect, cli) {
				if dep := nfOfURI(u, ns, byName); dep != "" && dep != nf {
					g.Add(n, rdf.C("dependsOnNF"), rdf.IRI(iri.NetworkFunction(s.Cluster, ns, dep)))
				}
			}
		}
		// --- PLMN et tranches, selon la NF (structure Open5GS v2.7)
		for _, p := range plmns(nf, sect) {
			pi := rdf.IRI(iri.PLMN(p[0], p[1]))
			g.Add(n, rdf.C("hasPlmn"), pi)
			g.Add(pi, rdf.Type, rdf.C("PLMN"))
			g.Add(pi, rdf.C("mcc"), rdf.Str(p[0]))
			g.Add(pi, rdf.C("mnc"), rdf.Str(p[1]))
		}
		for _, sst := range slices(nf, sect) {
			si := rdf.IRI(iri.Slice(sst, ""))
			g.Add(n, rdf.C("servesSlice"), si)
			g.Add(si, rdf.Type, rdf.C("Slice"))
			g.Add(si, rdf.C("sst"), rdf.Int(int64(sst)))
		}
	}
}

func iface(g *rdf.Graph, n rdf.Term, name string, port int64, proto string) {
	i := rdf.IRI(iri.Interface(n.Value(), name))
	g.Add(n, rdf.C("exposesInterface"), i)
	g.Add(i, rdf.Type, rdf.C("NetworkInterface"))
	g.Add(i, rdf.C("interfaceName"), rdf.Str(name))
	g.Add(i, rdf.C("port"), rdf.Int(port))
	g.Add(i, rdf.C("protocol"), rdf.Str(proto))
}

// nfConfig : contenu YAML de la ConfigMap montee par le Deployment (cle <nf>.yaml).
func nfConfig(d *unstructured.Unstructured, cms map[string]*unstructured.Unstructured, nf string) map[string]any {
	vols, _, _ := unstructured.NestedSlice(d.Object, "spec", "template", "spec", "volumes")
	for _, v := range vols {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name := str(m, "configMap", "name")
		if name == "" {
			continue
		}
		cm := cms[d.GetNamespace()+"/"+name]
		if cm == nil {
			continue
		}
		raw := str(cm.Object, "data", nf+".yaml")
		if raw == "" {
			continue
		}
		var out map[string]any
		if err := yaml.Unmarshal([]byte(raw), &out); err == nil {
			return out
		}
	}
	return nil
}

func firstServer(sect map[string]any, key string) map[string]any {
	m, _ := sect[key].(map[string]any)
	if m == nil {
		return nil
	}
	l, _ := m["server"].([]any)
	if len(l) == 0 {
		return nil
	}
	s, _ := l[0].(map[string]any)
	return s
}

func clientURIs(sect map[string]any, cli string) []string {
	sbi, _ := sect["sbi"].(map[string]any)
	c, _ := sbi["client"].(map[string]any)
	l, _ := c[cli].([]any)
	var out []string
	for _, e := range l {
		if m, ok := e.(map[string]any); ok {
			if u, _ := m["uri"].(string); u != "" {
				out = append(out, u)
			}
		}
	}
	return out
}

// nfOfURI : http://nrf.sylvatwin-5gc.svc.cluster.local:7777 -> "nrf" si un
// Deployment de NF du meme nom existe dans ce namespace.
func nfOfURI(u, ns string, byName map[string]string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	host := strings.Split(p.Hostname(), ".")
	if len(host) == 0 {
		return ""
	}
	svcNS := ns
	if len(host) > 1 && host[1] != "svc" {
		svcNS = host[1]
	}
	return byName[svcNS+"/"+host[0]]
}

func plmnOf(m map[string]any) (string, string, bool) {
	p, _ := m["plmn_id"].(map[string]any)
	if p == nil {
		return "", "", false
	}
	return numStr(p["mcc"]), numStr(p["mnc"]), true
}

// numStr : le YAML Open5GS ecrit mcc: 999, mnc: 70 (nombres). On garde la forme
// ecrite (sans ajouter de zero : "70" et "070" different en 3GPP).
func numStr(v any) string {
	switch x := v.(type) {
	case float64:
		return fmt.Sprintf("%d", int64(x))
	case string:
		return x
	}
	return fmt.Sprint(v)
}

func plmns(nf string, sect map[string]any) [][2]string {
	set := map[[2]string]bool{}
	add := func(l []any) {
		for _, e := range l {
			if m, ok := e.(map[string]any); ok {
				if mcc, mnc, ok := plmnOf(m); ok {
					set[[2]string{mcc, mnc}] = true
				}
			}
		}
	}
	switch nf {
	case "nrf":
		l, _ := sect["serving"].([]any)
		add(l)
	case "amf":
		l, _ := sect["plmn_support"].([]any)
		add(l)
		for _, k := range []string{"guami", "tai"} {
			l, _ := sect[k].([]any)
			add(l)
		}
	case "smf":
		info, _ := sect["info"].([]any)
		for _, e := range info {
			if m, ok := e.(map[string]any); ok {
				l, _ := m["tai"].([]any)
				add(l)
			}
		}
	}
	out := make([][2]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0]+out[i][1] < out[j][0]+out[j][1] })
	return out
}

func slices(nf string, sect map[string]any) []int {
	set := map[int]bool{}
	add := func(l []any) {
		for _, e := range l {
			if m, ok := e.(map[string]any); ok {
				if v, ok := m["sst"].(float64); ok {
					set[int(v)] = true
				}
			}
		}
	}
	switch nf {
	case "amf":
		ps, _ := sect["plmn_support"].([]any)
		for _, e := range ps {
			if m, ok := e.(map[string]any); ok {
				l, _ := m["s_nssai"].([]any)
				add(l)
			}
		}
	case "smf":
		info, _ := sect["info"].([]any)
		for _, e := range info {
			if m, ok := e.(map[string]any); ok {
				l, _ := m["s_nssai"].([]any)
				add(l)
			}
		}
	}
	out := make([]int, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
