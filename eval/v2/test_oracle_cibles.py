#!/usr/bin/env python3
"""Test du banc v2.1 : chaque cible de chaque oracle doit correspondre a un identifiant REELLEMENT produit.
  1. contre les identifiants de derive observes dans les donnees de campagne (cycles.jsonl) du scenario ;
  2. a defaut de donnees (scenario jamais execute), contre les identifiants que les regles construisent a partir des
     ressources des faits reels (out/facts.nt) : "drift/<Classe>/" + chemin de la ressource (cf. rules/derives/*.rq).
Usage : test_oracle_cibles.py [--version 2.1] [DOSSIER_CAMPAGNE ...]   ; code de sortie 1 s'il y a une cible orpheline."""
import json, os, re, sys
V2 = os.path.dirname(os.path.abspath(__file__))
FACTS = "/opt/nidar/sylvatwin-run/test2-sain-20261113/out/facts.nt"
ver = ""; args = sys.argv[1:]
if args[:1] == ["--version"]: ver = args[1]; args = args[2:]
camps = args or ["/opt/nidar/sylvatwin-run/eval-v2/campagne-v2"]
fn = "oracle-2.1.json" if ver == "2.1" else "oracle.json"
def observed(sc):
    ids = set()
    for c in camps:
        for r in (os.listdir(f"{c}/{sc}") if os.path.isdir(f"{c}/{sc}") else []):
            p = f"{c}/{sc}/{r}/sylvatwin/cycles.jsonl"
            if os.path.exists(p):
                for l in open(p):
                    try: ids |= set(json.loads(l)["ids"])
                    except Exception: pass
    return ids
res = []
if os.path.exists(FACTS):
    for l in open(FACTS):
        m = re.match(r"<https://w3id\.org/sylvatwin/res/([^>]+)> ", l)
        if m: res.append(m.group(1))
    res = sorted(set(res))
bad = 0; n = 0
for sc in sorted(os.listdir(V2 + "/scenarios")):
    p = f"{V2}/scenarios/{sc}/{fn}"
    if not os.path.exists(p): continue
    o = json.load(open(p)); obs = observed(sc)
    for d in o["attendu"]["derives"]:
        if not d["cible"]: continue            # cible vide : toute derive de la classe (InterNF, impacts globaux)
        n += 1
        a = [i for i in obs if i.split("/")[0] == d["classe"] and d["cible"] in i]
        if a: print(f"OK  {sc:3} {d['classe']}/{d['cible']} : {len(a)} identifiant(s) observe(s)"); continue
        if obs:
            print(f"ERR {sc:3} {d['classe']}/{d['cible']} : AUCUN identifiant observe ne correspond (observes de cette classe : {sorted(i for i in obs if i.split('/')[0] == d['classe'])[:2]})"); bad += 1; continue
        b = [r for r in res if d["cible"] in r]
        if b: print(f"OK  {sc:3} {d['classe']}/{d['cible']} : pas de donnees de campagne ; correspond a {len(b)} ressource(s) des faits reels (ex. {b[0][-60:]})")
        else: print(f"ERR {sc:3} {d['classe']}/{d['cible']} : ni observe, ni present dans les faits reels"); bad += 1
print(f"{n} cibles verifiees (oracle {ver or '2'}), {bad} orpheline(s)")
sys.exit(1 if bad else 0)
