#!/usr/bin/env python3
"""Analyse des repetitions (eval/runs/<scenario>/<n>/) : premiere detection,
delai, derives, NF touchees (chemin), verdicts, resolution ; vue Flux et kubectl.
Comparaison a l'oracle de eval/criteres.md. Sortie : eval/resultats.md + .json.
Lecture seule (fichiers de journal)."""
import glob, json, os, re, statistics, sys
from datetime import datetime

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
RUNS = os.path.join(ROOT, "runs")
W = "my-workload-kubeadm-capd/sylvatwin-5gc/"

def t(s):
    s = s.strip().replace("Z", "+00:00")
    if "." in s:
        a, b = s.split("."); frac, tz = b[:b.index("+")], b[b.index("+"):]
        s = a + "." + frac[:6].ljust(6, "0") + tz
    return datetime.fromisoformat(s)

DEP = ["amf", "ausf", "bsf", "nssf", "pcf", "smf", "udm", "udr"]
ORACLE = {
  "S0-A": dict(drift={"SpecDrift": "Deployment/nrf"}, nf={("nrf", "direct")} | {(n, "dependance") for n in DEP}, verdict="SuspectedViolation", flux_fix=True),
  "S3-b": dict(drift={"SpecDrift": "Deployment/nrf"}, nf={("nrf", "direct")} | {(n, "dependance") for n in DEP}, verdict="SuspectedViolation", flux_fix=True),
  "S0-B": dict(drift={"ConvergenceDrift": "Deployment/udm", "BrokenReferenceDrift": "Secret/udm-keys"}, nf={("udm", "direct")}, verdict=None, flux_fix=False),
  "S2":   dict(drift={"BrokenReferenceDrift": "Secret/udm-keys"}, nf={("udm", "direct")}, verdict="ConfirmedViolation", flux_fix=False),
  "S3-a": dict(drift={"BrokenReferenceDrift": "Secret/udm-keys"}, nf={("udm", "direct")}, verdict="ConfirmedViolation", flux_fix=False),
  "S1":   dict(drift={"PlacementDrift": "Pod/bsf-"}, nf={("bsf", "direct")}, verdict="ConfirmedViolation", flux_fix=False),
  "S3-c": dict(drift={"PlacementDrift": "Pod/bsf-"}, nf={("bsf", "direct")}, verdict="ConfirmedViolation", flux_fix=False),
  "S4-a": dict(drift={"InterNFConsistencyDrift": "plmn/"}, nf=set(), verdict="ConfirmedViolation", flux_fix=True),
  "S4-b": dict(drift={"InterNFConsistencyDrift": "slice/"}, nf=set(), verdict="ConfirmedViolation", flux_fix=False),
  "S5":   dict(drift={"GitOpsSyncDrift": "Kustomization"}, nf=None, verdict="ConfirmedViolation", flux_fix=False),
  "B1":   dict(drift={}, nf=set(), verdict=None, flux_fix=None),
  "B2":   dict(drift={}, nf=set(), verdict=None, flux_fix=None),
}

def nt_rows(path):
    rows = []
    if not os.path.exists(path):
        return rows
    for l in open(path):
        m = re.match(r'<([^>]*)> <([^>]*)> (?:<([^>]*)>|"((?:[^"\\]|\\.)*)"(?:\^\^<[^>]*>)?) \.', l.strip())
        if m:
            rows.append((m.group(1), m.group(2), m.group(3) if m.group(3) is not None else m.group(4)))
    return rows

C = "https://w3id.org/sylvatwin/core#"
RDFT = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

def cycle_info(cdir):
    j = json.load(open(os.path.join(cdir, "cycle.json")))
    drifts = nt_rows(os.path.join(cdir, "drifts.nt"))
    verd = nt_rows(os.path.join(cdir, "verdicts.nt"))
    types = {}
    for s, p, o in drifts:
        if p == RDFT:
            types[s] = o.replace(C, "")
    nfs = set()
    for s, ty in types.items():
        if ty == "FunctionImpactDrift":
            f = [o for (s2, p, o) in drifts if s2 == s and p == C + "affectsFunction"]
            pth = [o for (s2, p, o) in drifts if s2 == s and p == C + "impactPath"]
            if f and pth:
                nfs.add((f[0].rsplit("/", 1)[-1], pth[0]))
    vby = {}
    for s, p, o in verd:
        if p == RDFT:
            d = s.replace("https://w3id.org/sylvatwin/verdict/", "")
            vby[d] = o.replace(C, "")
    return dict(start=t(j["start"]), clusters=j["clusters"], types=types, nfs=nfs, verdicts=vby)

def analyse(sc, n):
    d = os.path.join(RUNS, sc, str(n))
    if not os.path.exists(os.path.join(d, "injection.txt")):
        return None
    inj = t(open(os.path.join(d, "injection.txt")).read())
    cycles = []
    for cd in sorted(glob.glob(os.path.join(d, "cycles", "cycle-*"))):
        if os.path.isdir(cd) and os.path.exists(os.path.join(cd, "cycle.json")):
            try:
                cycles.append(cycle_info(cd))
            except Exception:
                pass
    o = ORACLE[sc]
    before = [c for c in cycles if c["start"] < inj]
    after = [c for c in cycles if c["start"] >= inj]
    pre_drift = sum(len(c["types"]) for c in before)
    want = o["drift"]
    def match(c):
        ok = True
        for ty, frag in want.items():
            ok &= any(v == ty and frag in s for s, v in c["types"].items())
        return ok and bool(want)
    first = next((c for c in after if match(c)), None)
    res = dict(scenario=sc, rep=n, injection=inj.isoformat(), cycles=len(cycles), cycles_avant_avec_derive=pre_drift)
    if first:
        res["detecte"] = True
        res["delai_s"] = round((first["start"] - inj).total_seconds(), 1)
        res["derives"] = sorted(set(first["types"].values()))
        res["nf"] = sorted(first["nfs"])
        main_ty = list(want)[0]
        main = [s for s, v in first["types"].items() if v == main_ty and want[main_ty] in s]
        vkey = main[0].replace("https://w3id.org/sylvatwin/drift/", "") if main else ""
        res["verdict_premier"] = first["verdicts"].get(vkey)
        seq = []
        for c in after:
            v = c["verdicts"].get(vkey)
            if v and (not seq or seq[-1][1] != v):
                seq.append((round((c["start"] - inj).total_seconds()), v))
        res["sequence_verdict"] = seq
        gone = next((c for c in after if c["start"] > first["start"] and not match(c)), None)
        res["resolu_apres_s"] = round((gone["start"] - inj).total_seconds(), 1) if gone else None
        res["tous_verdicts_premier"] = dict(sorted(first["verdicts"].items()))
    else:
        res["detecte"] = False
        alltypes = set()
        for c in after:
            alltypes |= set(c["types"].values())
        res["derives_vues"] = sorted(alltypes)
        conf = sum(1 for c in after for v in c["verdicts"].values() if v == "ConfirmedViolation")
        susp = sum(1 for c in after for v in c["verdicts"].values() if v == "SuspectedViolation")
        res["confirmees_bénin"] = conf
        res["suspectees_bénin"] = susp
    # Flux et kubectl
    ev = open(os.path.join(d, "flux-events.txt")).read() if os.path.exists(os.path.join(d, "flux-events.txt")) else ""
    lg = open(os.path.join(d, "flux-kustomize-controller.log")).read() if os.path.exists(os.path.join(d, "flux-kustomize-controller.log")) else ""
    res["flux_log_lignes"] = len([l for l in lg.splitlines() if l.strip()])
    configured = re.findall(r'"([A-Za-z]+/[a-z0-9.-]+/[a-z0-9.-]+)":"configured"', lg) + re.findall(r'([A-Z][a-z]+/[a-z0-9-]+/[a-z0-9.-]+) configured', lg)
    res["flux_objets_reconfigures"] = sorted(set(configured))
    obs = open(os.path.join(d, "observation.txt")).read() if os.path.exists(os.path.join(d, "observation.txt")) else ""
    res["kubectl"] = open(os.path.join(d, "kubectl.txt")).read()[-1500:] if os.path.exists(os.path.join(d, "kubectl.txt")) else ""
    res["observation_extrait"] = obs[:600]
    j = open(os.path.join(d, "journal.txt")).read()
    res["retour_sain"] = "retour a l'etat sain : OK" in j
    return res

def main():
    out = {}
    for sc in ORACLE:
        out[sc] = [r for r in (analyse(sc, n) for n in (1, 2, 3)) if r]
    json.dump(out, open(os.path.join(ROOT, "resultats.json"), "w"), indent=1, default=str)
    for sc, reps in out.items():
        if not reps:
            continue
        print(f"== {sc}")
        for r in reps:
            if r["detecte"]:
                print(f"  rep {r['rep']} : detecte en {r['delai_s']} s ; derives {r['derives']} ; NF {r['nf']} ; verdict {r['verdict_premier']} ; seq {r['sequence_verdict']} ; resolu {r['resolu_apres_s']} s ; flux reconf {r['flux_objets_reconfigures']} ; sain {r['retour_sain']}")
            else:
                print(f"  rep {r['rep']} : non detecte ; derives vues {r['derives_vues']} ; confirmees {r['confirmees_bénin']} suspectees {r['suspectees_bénin']} ; sain {r['retour_sain']}")

if __name__ == "__main__":
    main()
