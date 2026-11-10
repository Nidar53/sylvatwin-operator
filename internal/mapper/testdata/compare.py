#!/usr/bin/env python3
"""BARRIERE 1 : compare la sortie de l'operateur (N-Triples) a l'exemple de
reference de sylvatwin-poc (Turtle), hors faits volatils, et explique chaque ecart.

Normalisation (documentee dans ecarts.md) :
  - observedAt, lastSyncTime : horodatages de la photo, retires des deux cotes ;
  - desiredRevision, actualRevision : revisions Git (l'exemple date du commit
    9099eec, la photo du commit 6f6fb11) ; retirees des deux cotes ;
  - l'exemple est un SOUS-ENSEMBLE choisi a la main (5 NF, pas tous les labels
    de pods...) : on compare donc sur les SUJETS de l'exemple, et on liste a
    part ce que l'operateur ecrit EN PLUS (attendu : faits non retenus dans
    l'exemple, jamais de faits faux).
Sortie : code 0 si, sur les sujets de l'exemple, les deux graphes sont egaux.
"""
import sys
from rdflib import Graph, URIRef, Literal, RDF, RDFS
from rdflib.compare import to_isomorphic, graph_diff

op_path, ex_path = sys.argv[1], sys.argv[2]
C = "https://w3id.org/sylvatwin/core#"
VOLATILE = {URIRef(C + "observedAt"), URIRef(C + "lastSyncTime"), URIRef(C + "desiredRevision"), URIRef(C + "actualRevision")}

op = Graph().parse(op_path, format="nt")
ex = Graph().parse(ex_path, format="turtle")

def norm(g):
    h = Graph()
    for s, p, o in g:
        if p in VOLATILE:
            continue
        if isinstance(o, Literal) and o.datatype is None and o.language is None:
            o = Literal(str(o))
        h.add((s, p, o))
    return h

op, ex = norm(op), norm(ex)
# Faits VRAIS que l'exemple (ecrit a la main le 08/11) n'avait pas retenus.
# Chacun est justifie dans testdata/ecarts.md et verifie sur la photo.
DOCUMENTED = [
    ("res:management-cluster/sylva-system/Cluster.cluster.x-k8s.io/management-cluster", ":managedBy", "res:management-cluster/sylva-system/HelmRelease.helm.toolkit.fluxcd.io/cluster"),
    ("res:management-cluster/sylva-system/Cluster.cluster.x-k8s.io/management-cluster", ":ready", "true"),
    ("res:management-cluster/sylvatwin-flux/GitRepository.source.toolkit.fluxcd.io/sylvatwin-poc", ":reconcileTimeout", "60.0"),
    ("res:my-workload-kubeadm-capd/sylvatwin-5gc/NetworkFunction/smf", ":exposesInterface", "res:my-workload-kubeadm-capd/sylvatwin-5gc/NetworkFunction/smf/interface/N3"),
    ("res:my-workload-kubeadm-capd/sylvatwin-5gc/Secret/udm-keys", ":hasLabel", "res:my-workload-kubeadm-capd/sylvatwin-5gc/Secret/udm-keys/label/app.kubernetes.io%2Fpart-of"),
    # v1.3.0 (partie 3, 10/11/2026) : mountsConfigMap, progressDeadlineSeconds, startedAt
] + [
    (f"res:my-workload-kubeadm-capd/sylvatwin-5gc/Deployment/{nf}", ":mountsConfigMap", f"res:my-workload-kubeadm-capd/sylvatwin-5gc/ConfigMap/{nf}-config")
    for nf in ("amf", "bsf", "nrf", "smf", "udm")
] + [
    (f"res:my-workload-kubeadm-capd/sylvatwin-5gc/Deployment/{nf}", ":progressDeadlineSeconds", "600.0")
    for nf in ("amf", "bsf", "nrf", "smf", "udm")
] + [
    (f"res:my-workload-kubeadm-capd/sylvatwin-5gc/Pod/{pod}", ":startedAt", "2026-10-27T13:40:28+00:00")
    for pod in ("bsf-78db7b6587-8wkqn", "nrf-57b6849ccd-wflfd", "udm-7cf6c5b78d-rw6fk")
]
subjects = set(ex.subjects())

def restrict(g, subs):
    h = Graph()
    for s, p, o in g:
        if s in subs:
            h.add((s, p, o))
    return h

op_r, ex_r = restrict(op, subjects), ex
both, only_op, only_ex = graph_diff(to_isomorphic(op_r), to_isomorphic(ex_r))
short = lambda t: str(t).replace("https://w3id.org/sylvatwin/res/", "res:").replace(C, ":")
print(f"operateur : {len(op)} triplets ; exemple : {len(ex)} triplets ; sujets de l'exemple : {len(subjects)}")
print(f"sur les sujets de l'exemple : communs={len(both)}  seulement operateur={len(only_op)}  seulement exemple={len(only_ex)}")
for title, g in [("SEULEMENT DANS L'EXEMPLE (manquants cote operateur)", only_ex),
                 ("SEULEMENT DANS L'OPERATEUR (sur les sujets de l'exemple)", only_op)]:
    if len(g):
        print("--", title)
        for s, p, o in sorted(g, key=lambda t: (str(t[0]), str(t[1]), str(t[2]))):
            print("  ", short(s), short(p), short(o))
extra = set(op.subjects()) - subjects
kinds = {}
for s in extra:
    for t in op.objects(s, RDF.type):
        kinds[short(t)] = kinds.get(short(t), 0) + 1
print("-- sujets ecrits EN PLUS par l'operateur (hors exemple, par type) :", dict(sorted(kinds.items())))
shortset = {(short(s), short(p), short(o)) for s, p, o in only_op}
undocumented = shortset - set(DOCUMENTED)
print(f"-- ecarts operateur documentes (ecarts.md) : {len(shortset & set(DOCUMENTED))} ; NON documentes : {len(undocumented)}")
sys.exit(0 if not undocumented and len(only_ex) == 0 else 1)
