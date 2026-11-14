#!/usr/bin/env python3
"""Banc v2.1 : extrait, en LECTURE SEULE, le journal du kustomize-controller pour la Kustomization
sylvatwin-flux/sylvatwin-5gc. Usage : flux_log.py DEPUIS_UTC SORTIE.jsonl
Ne garde que : ts, niveau, message (tronque), revision, objets dont l'action n'est pas "unchanged", erreur (tronquee).
Aucun Secret n'est lu : le journal ne contient que des noms d'objets et des actions."""
import json, os, subprocess, sys
since, out = sys.argv[1], sys.argv[2]
env = dict(os.environ); env["PATH"] = "/opt/nidar/go/bin:/opt/nidar/sylva-core/bin:" + env.get("PATH", "")
env["NO_PROXY"] = env["no_proxy"] = "localhost,127.0.0.1,172.30.0.0/24,100.72.0.0/16,100.73.0.0/16,100.100.0.0/16,100.96.0.0/16,.sylva,.svc,.cluster.local,.cluster.local.,.orange-labs.fr,.intraorange,.francetelecom.fr,.tech.orange"
r = subprocess.run(["kubectl", "--kubeconfig", "/opt/nidar/sylva-core/management-cluster-kubeconfig", "-n", "flux-system",
                    "logs", "deploy/kustomize-controller", "--since-time=" + since], capture_output=True, text=True, timeout=180, env=env)
n = 0
with open(out, "w") as f:
    for l in r.stdout.splitlines():
        try: d = json.loads(l)
        except Exception: continue
        if d.get("name") != "sylvatwin-5gc" or d.get("namespace") != "sylvatwin-flux": continue
        rec = dict(ts=d["ts"], level=d["level"], msg=d["msg"][:160], revision=d.get("revision"),
                   changed={k: v for k, v in (d.get("output") or {}).items() if v != "unchanged"})
        if d.get("error"): rec["error"] = str(d["error"])[:300]
        f.write(json.dumps(rec) + "\n"); n += 1
print(n, "lignes ->", out)
