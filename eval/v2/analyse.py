#!/usr/bin/env python3
"""Analyse v2 / v2.1 : lit un dossier de campagne (campagne.py) et ecrit resultats-v2.md / .json (oracle 2) ou resultats-v2.1.md / .json (--version 2.1).
Usage : analyse.py [--version 2.1] DOSSIER [DOSSIER...]   (lecture seule des fichiers de mesure ; aucun acces cluster)
La version 2 reste reproductible telle quelle ; la version 2.1 lit scenarios/<id>/oracle-2.1.json et ajoute le journal du kustomize-controller."""
import json, os, re, statistics, sys, datetime as dt
V2 = os.path.dirname(os.path.abspath(__file__))
VER = ""        # "" = oracle 2 ; "2.1" = oracle 2.1
def orc_path(sc): return f"{V2}/scenarios/{sc}/" + ("oracle-2.1.json" if VER == "2.1" else "oracle.json")
def load_orc(sc): return json.load(open(orc_path(sc)))
def P(s): return dt.datetime.fromisoformat(s.replace("Z", "+00:00"))
_FL = {}
def flux_log(d):
    """journal du kustomize-controller de la campagne (flux_log.py), mis en cache."""
    camp = os.path.dirname(os.path.dirname(d))
    if camp not in _FL:
        _FL[camp] = [dict(e, t=P(e["ts"])) for e in jl(camp + "/flux-kustomize-controller.jsonl")]
    return _FL[camp]
def jl(p):
    out = []
    if os.path.exists(p):
        for l in open(p):
            try: out.append(json.loads(l))
            except Exception: pass
    return out
RANK = {"SuspectedViolation": 1, "ConfirmedViolation": 2, "UnknownVerdict": 0}
def med(v): return statistics.median(v) if v else None
def fmt_d(v):
    if not v: return "—"
    return "%.0f s (%.0f–%.0f)" % (med(v), min(v), max(v))

def analyse_rep(d, scen):
    orc = load_orc(scen)
    marks = [l.split("\t") for l in open(d + "/marks.tsv").read().splitlines()] if os.path.exists(d + "/marks.tsv") else []
    M = {}
    for t, e in marks: M.setdefault(e.split(" ")[0], t)
    t0s = M.get("t0_push_debut") or M.get("t0")
    if not t0s: return None
    t0 = P(t0s); tend = P(M["fin_attente"]) if "fin_attente" in M else None; trest = P(M["restauration_faite"]) if "restauration_faite" in M else None
    tsain = P(M["etat_sain_oui"]) if "etat_sain_oui" in M else None
    R = dict(scenario=scen, t0=t0s, git=orc["git"], benin=orc["benin"], dans_le_modele=orc["dans_le_modele"], couche=orc["couche"], source=orc["source_erreur"])
    base = json.load(open(d + "/baseline-avant.json"))["ids"] if os.path.exists(d + "/baseline-avant.json") else {}
    cyc = [c for c in jl(d + "/sylvatwin/cycles.jsonl") if P(c["t"]) > t0 and (not trest or P(c["t"]) <= trest)]
    new = {}   # id -> dict(class, first, verdicts[(t,v)])
    for c in cyc:
        for i, cl in c["ids"].items():
            if i in base: continue
            e = new.setdefault(i, dict(cls=cl, first=P(c["t"]), vs=[]))
            e["vs"].append((P(c["t"]), c["ver"].get(i)))
    att = orc["attendu"]
    # derives attendues
    dets = []
    for e in att.get("derives", []):
        m = [i for i, v in new.items() if v["cls"] == e["classe"] and (not e["cible"] or e["cible"] in i)]
        r = dict(classe=e["classe"], cible=e["cible"], detectee=bool(m), mere_attendue=e["mere"])
        if m:
            first = min(new[i]["first"] for i in m); r["delai_s"] = (first - t0).total_seconds()
            vi = [v for t, v in sorted((t, v) for i in m for t, v in new[i]["vs"])]
            r["verdict_initial"] = vi[0]; r["verdict_initial_ok"] = (vi[0] == e["verdict_initial"])
            if e.get("verdict_final"):
                conf = [t for i in m for t, v in new[i]["vs"] if v == e["verdict_final"]]
                r["verdict_final_atteint"] = bool(conf)
                if conf: r["verdict_final_apres_s"] = (min(conf) - first).total_seconds()
        dets.append(r)
    R["derives_attendues"] = dets
    # impacts
    got = set()
    for i, v in new.items():
        if v["cls"] == "FunctionImpactDrift":
            p = i.split("/"); got.add((p[1], p[-1]))
    exp = set()
    for im in att.get("impacts", []):
        for nf in im["nf"]: exp.add((im["cause"], nf))
    R["impacts_attendus"] = len(exp); R["impacts_observes"] = len(got & exp); R["impacts_manquants"] = sorted("%s/%s" % x for x in exp - got)
    # fausses alertes
    tol = {t["classe"]: t["verdict_max"] for t in att.get("derives_transitoires", [])}
    expcls = [(e["classe"], e["cible"]) for e in att.get("derives", [])]
    fa = []
    for i, v in new.items():
        cl = v["cls"]
        if any(cl == c and (not t or t in i) for c, t in expcls): continue
        if cl == "FunctionImpactDrift":
            p = i.split("/")
            if (p[1], p[-1]) in exp: continue
            if "FunctionImpactDrift" in tol or any(p[1] == c for c in tol):
                if max(RANK.get(x, 0) for _, x in v["vs"]) <= RANK[tol.get("FunctionImpactDrift", "SuspectedViolation")]: continue
            elif any(p[1] == c and (not t or True) for c, t in expcls) and False: continue
        elif cl in tol:
            if max(RANK.get(x, 0) for _, x in v["vs"]) <= RANK[tol[cl]]: continue
        fa.append(dict(id=i.replace("my-workload-kubeadm-capd%2Fmy-workload-kubeadm-capd/", "wl/"), classe=cl, verdict_max=max((x for _, x in v["vs"] if x), key=lambda x: RANK.get(x, 0), default=None)))
    R["fausses_alertes"] = fa; R["nouvelles_derives"] = len(new); R["transitoires_toleres"] = sum(1 for i, v in new.items() if v["cls"] in tol and not any(f["id"] == i for f in fa))
    allexp_ok = all(x["detectee"] and x.get("verdict_initial_ok", False) and x.get("verdict_final_atteint", True) for x in dets)
    R["sylvatwin_detecte"] = bool(dets) and all(x["detectee"] for x in dets) if dets else None
    R["sylvatwin_detecte_au_moins_une"] = any(x["detectee"] for x in dets) if dets else None
    R["verdict_conforme"] = (allexp_ok and not fa) if dets else (not fa)
    R["delai_sylvatwin_s"] = min((x["delai_s"] for x in dets if x.get("delai_s") is not None), default=None)
    # resolu avant restauration ?
    if cyc and tend:
        pre = [c for c in cyc if P(c["t"]) <= tend]
        R["resolu_avant_restauration"] = bool(pre) and set(i for i in pre[-1]["ids"] if i not in base) == set() and bool(new)
    # Flux
    ev = []
    if os.path.exists(d + "/flux/events.log"):
        for l in open(d + "/flux/events.log"):
            try: ts, rest = l.rstrip("\n").split("\t", 1)
            except ValueError: continue
            p = rest.split(None, 4)
            if len(p) >= 4: ev.append(dict(t=P(ts), kind=p[0], name=p[1], typ=p[2], reason=p[3], msg=p[4] if len(p) > 4 else ""))
    poll = jl(d + "/flux/poll.jsonl")
    win = lambda t: t > t0 and (not tend or t <= tend)
    fsig = None
    for e in ev:
        if not win(e["t"]) or e["name"] != "sylvatwin-5gc": continue
        chg = re.search(r"configured|created|deleted", e["msg"]) and not orc["git"]
        if e["typ"] == "Warning" or chg: fsig = e["t"]; break
    for p in poll:
        if win(P(p["t"])) and p.get("ready") and p["ready"][0] == "False" and (fsig is None or P(p["t"]) < fsig): fsig = P(p["t"]); break
    R["flux_signale"] = fsig is not None; R["flux_delai_s"] = (fsig - t0).total_seconds() if fsig else None
    fcor = None
    if not orc["git"]:
        for p in poll:
            if win(P(p["t"])) and p.get("probe") == "0": fcor = P(p["t"]); break
    R["flux_corrige"] = fcor is not None if not orc["git"] else None; R["flux_correction_delai_s"] = (fcor - t0).total_seconds() if fcor else None
    if orc["git"]:
        ap = [P(p["t"]) for p in poll if win(P(p["t"])) and p.get("applied") and p.get("artifact") and p["applied"] == p["artifact"] and p.get("probe") == "0"]
        R["flux_applique_commit_s"] = (min(ap) - t0).total_seconds() if ap else None
    # ---- v2.1 : journal du kustomize-controller (source retenue), delais Git, verdicts multiples ----
    fl = [e for e in flux_log(d) if e["t"] > t0 and (not tend or e["t"] <= tend)]
    R["flux_journal_disponible"] = bool(flux_log(d))
    err = [e for e in fl if e["level"] == "error"]
    R["flux_journal_erreur"] = bool(err); R["flux_journal_erreur_delai_s"] = (err[0]["t"] - t0).total_seconds() if err else None
    R["flux_journal_suspendue"] = any("suspended" in e["msg"] for e in fl)
    fo = orc.get("flux_objet")
    chg = [e for e in fl if fo and fo in e["changed"]] if fo else []
    R["flux_objet"] = fo
    R["flux_journal_corrige_objet"] = (bool(chg) if fo else None); R["flux_journal_corrige_delai_s"] = (chg[0]["t"] - t0).total_seconds() if chg else None
    R["flux_signale_total"] = bool(R["flux_signale"] or err)
    sha = None
    for l in open(d + "/marks.tsv"):
        if "\tpush_fait " in l: sha = l.split("push_fait ")[1].strip()
    if orc["git"] and sha:
        rev = "test@sha1:" + sha
        okr = [e for e in flux_log(d) if e.get("revision") == rev and e["level"] == "info" and e["msg"].startswith("Reconciliation finished")]
        ko = [e for e in flux_log(d) if e.get("revision") == rev and e["level"] == "error"]
        R["flux_applique_instant"] = okr[0]["ts"] if okr else None
        R["flux_applique_depuis_push_s"] = (okr[0]["t"] - t0).total_seconds() if okr else None
        R["flux_premiere_tentative_en_echec_s"] = (ko[0]["t"] - t0).total_seconds() if ko and not okr else None
        ref = okr[0]["t"] if okr else (ko[0]["t"] if ko else None)
        R["flux_reference_depuis"] = "application" if okr else ("premiere tentative en echec" if ko else None)
        dd = [x["delai_s"] for x in dets if x.get("delai_s") is not None]
        R["delai_sylvatwin_depuis_push_s"] = min(dd) if dd else None
        R["delai_sylvatwin_depuis_flux_s"] = (min(dd) - (ref - t0).total_seconds()) if dd and ref else None
    mu = {}
    for c in cyc:
        for i, v in (c.get("ver_multi") or {}).items(): mu[i] = v
    R["verdicts_multiples"] = sorted(i.split("/")[0] + "/" + i.split("/")[-1] for i in mu)
    # diffs
    for lab, key in (("t0+10s", "10"), ("avant-restauration", "avant")):
        for tool in ("kubectl", "flux"):
            f = f"{d}/diffs/{lab}-{tool}.txt"
            if os.path.exists(f):
                t = open(f).read().splitlines(); rc = t[0] if t else "rc=?"
                body = [x for x in t[1:] if x[:1] in "+-"]
                R[f"diff_{tool}_{key}"] = dict(rc=rc, lignes=len(body), non_vide=bool(body), erreur=(rc not in ("rc=0", "rc=1")))
    # alertes Prometheus
    al = jl(d + "/alerts/poll.jsonl"); basenames = set(); newal = {}
    for p in al:
        if not p.get("alerts"): continue
        for a in p["alerts"]:
            if P(p["t"]) <= t0: basenames.add(a["name"])
            elif win(P(p["t"])) and a["name"] not in basenames and (a.get("ns") in ("sylvatwin-5gc", None) or "sylvatwin" in str(a.get("dep"))):
                newal.setdefault(a["name"], P(p["t"]))
    R["prom_alertes"] = sorted(newal); R["prom_detecte"] = bool(newal)
    R["prom_delai_s"] = min(((t - t0).total_seconds() for t in newal.values()), default=None)
    R["retour_sain_s"] = (tsain - trest).total_seconds() if tsain and trest else None
    R["etat_sain_apres"] = "etat_sain_oui" in M
    return R

def main():
    global VER
    args = sys.argv[1:]
    if args[:1] == ["--version"]: VER = args[1]; args = args[2:]
    dirs = args
    reps = []
    for D in dirs:
        for sc in sorted(os.listdir(D)):
            if not os.path.isdir(f"{D}/{sc}") or not os.path.exists(orc_path(sc)): continue
            for r in sorted(os.listdir(f"{D}/{sc}")):
                if r.startswith("rep") and os.path.exists(f"{D}/{sc}/{r}/marks.tsv"):
                    x = analyse_rep(f"{D}/{sc}/{r}", sc)
                    if x: x["rep"] = int(r[3:]); reps.append(x)
    out = dirs[0]
    if VER == "2.1": return report21(reps, out)
    json.dump(reps, open(out + "/resultats-v2.json", "w"), indent=1, default=str)
    by = {}
    for r in reps: by.setdefault(r["scenario"], []).append(r)
    L = ["# Résultats v2 — SylvaTwin face à Flux CD (PRÉLIMINAIRES)", "", f"Dossier : `{out}` ; {len(reps)} répétitions. Oracle : `eval/v2/criteres-v2.md` (écrit avant tout essai).",
         "Horloge unique : serveur. Délais : t0 = instant d'injection (push Git pour F3, N2, B3). **Mesuré** = observé dans les journaux ; les attendus de Flux sont SUPPOSÉS.", "",
         "| id | couche | source de l'erreur | SylvaTwin détecte | délai SylvaTwin | verdict conforme | fausses alertes (total) | Flux signale | Flux corrige | kubectl diff -k | flux diff | Alertmanager | retour sain |", "|---|---|---|---|---|---|---|---|---|---|---|---|---|"]
    tool = {t: [0, 0, 0] for t in ("sylvatwin", "flux", "kdiff", "fdiff", "prom")}   # tp, malveillants, fp(benins)
    for sc in sorted(by, key=lambda s: (load_orc(s)["couche"] == "benin", s)):
        rr = by[sc]; n = len(rr); r0 = rr[0]
        k = lambda f: sum(1 for r in rr if f(r))
        dl = [r["delai_sylvatwin_s"] for r in rr if r.get("delai_sylvatwin_s") is not None]
        sdet = k(lambda r: r["sylvatwin_detecte"]) if r0["sylvatwin_detecte"] is not None else None
        fa = sum(len(r["fausses_alertes"]) for r in rr)
        fs = k(lambda r: r["flux_signale"]); fc = k(lambda r: r["flux_corrige"]) if r0["flux_corrige"] is not None else None
        kd = k(lambda r: r.get("diff_kubectl_10", {}).get("non_vide")); fd = k(lambda r: r.get("diff_flux_10", {}).get("non_vide"))
        fd_err = all(r.get("diff_flux_10", {}).get("erreur", True) for r in rr)
        pr = k(lambda r: r["prom_detecte"]); vc = k(lambda r: r["verdict_conforme"]); rs = [r["retour_sain_s"] for r in rr if r.get("retour_sain_s") is not None]
        dag = "†" if r0["git"] else ""
        L.append(f"| {sc} | {r0['couche']} | {r0['source']} | {('%d/%d' % (sdet, n)) if sdet is not None else 'sans objet (rien attendu)'} | {fmt_d(dl)} | {vc}/{n} | {fa} | {fs}/{n} | {('%d/%d' % (fc, n)) if fc is not None else 'n/a (Git)'} | {kd}/{n}{dag} | {('n/c (erreur : flux diff ne suit pas kubeConfig)' if fd_err else '%d/%d%s' % (fd, n, dag))} | {pr}/{n} | {fmt_d(rs)} |")
        mal = not r0["benin"]
        for key, val in (("sylvatwin", sdet if sdet is not None else 0), ("flux", fs), ("kdiff", kd), ("fdiff", fd), ("prom", pr)):
            if key in ("kdiff", "fdiff") and r0["git"]: continue
            if key == "fdiff" and fd_err: continue
            if mal: tool[key][0] += val; tool[key][1] += n
            else: tool[key][2] += val
    L += ["", "† Scénario Git : le diff compare le dépôt (déjà poussé) au cluster ; il montre le commit en attente, pas une dérive : exclu des rappels `kubectl diff` / `flux diff`.", ""]
    L += ["## Rappel et précision globaux (scénarios malveillants ; les bénins B1–B4 comptent comme faux positifs)", "", "| outil | détections | répétitions malveillantes | rappel | détections sur bénins | précision |", "|---|---|---|---|---|---|"]
    names = dict(sylvatwin="SylvaTwin", flux="Flux CD (événement ou Ready=False ou correction)", kdiff="kubectl diff -k", fdiff="flux diff kustomization", prom="Alertmanager (règles installées)")
    for key, (tp, nm, fp) in tool.items():
        if key == "fdiff" and nm == 0:
            L.append(f"| {names[key]} | — | — | **non comparé** : `flux diff` évalue la Kustomization contre le cluster de management (elle vise un cluster distant par `spec.kubeConfig`) ; erreur « namespace introuvable » | — | — |"); continue
        L.append(f"| {names[key]} | {tp} | {nm} | {('%.0f %%' % (100 * tp / nm)) if nm else '—'} | {fp} | {('%.0f %%' % (100 * tp / (tp + fp))) if tp + fp else '—'} |")
    mod = [r for r in reps if not r["benin"] and r["dans_le_modele"]]
    if mod:
        tp = sum(1 for r in mod if r["sylvatwin_detecte"]); L += ["", f"Rappel SylvaTwin **dans le modèle** (hors H1, H2) : {tp}/{len(mod)}."]
    allnew = sum(r["nouvelles_derives"] for r in reps); allfa = sum(len(r["fausses_alertes"]) for r in reps)
    L += [f"Précision SylvaTwin par dérive (nouvelles dérives conformes à l'oracle ou tolérées / toutes les nouvelles) : {allnew - allfa}/{allnew}.", "", "Analyseur statique : **non comparé** (aucun installé).", ""]
    L += ["## Détail par répétition (impacts, fausses alertes)", ""]
    for r in sorted(reps, key=lambda r: (r["scenario"], r["rep"])):
        L.append(f"* **{r['scenario']} rép {r['rep']}** — dérives attendues : " + ", ".join(f"{x['classe']} {'détectée en %.0f s (%s)' % (x['delai_s'], x.get('verdict_initial')) if x['detectee'] else 'NON détectée'}" + (f", final {'atteint' if x.get('verdict_final_atteint') else 'non atteint'}" + (f" après {x['verdict_final_apres_s']:.0f} s" if x.get('verdict_final_apres_s') else "") if 'verdict_final_atteint' in x else "") for x in r["derives_attendues"]) + f" ; impacts {r['impacts_observes']}/{r['impacts_attendus']}" + (f" (manquants : {', '.join(r['impacts_manquants'][:4])}…)" if r["impacts_manquants"] else "") + f" ; fausses alertes : {len(r['fausses_alertes'])}" + (" (" + "; ".join(f['classe'] + ' ' + str(f['verdict_max']) for f in r['fausses_alertes'][:4]) + ")" if r["fausses_alertes"] else "") + f" ; transitoires tolérés : {r['transitoires_toleres']} ; état sain après : {r['etat_sain_apres']}.")
    open(out + "/resultats-v2.md", "w").write("\n".join(L) + "\n")
    print("\n".join(L[:30]))
def report21(reps, out):
    json.dump(reps, open(out + "/resultats-v2.1.json", "w"), indent=1, default=str)
    by = {}
    for r in reps: by.setdefault(r["scenario"], []).append(r)
    order = sorted(by, key=lambda s: (load_orc(s)["couche"] == "benin", s))
    f = lambda n, d: "%d/%d" % (n, d)
    L = ["# Résultats v2.1 — SylvaTwin face à Flux CD (PRÉLIMINAIRES)", "",
         f"Dossier : `{out}` ; {len(reps)} répétitions ; **mêmes données** que `resultats-v2.md` (conservé tel quel). Oracle : version 2.1 (`scenarios/*/oracle-2.1.json`, note datée du 14/11/2026 dans `criteres-v2.1.md`).",
         "Horloge unique : serveur (UTC). t0 = injection (push Git pour F3, N2, B3 ; retrait du label pour K4). **Mesuré** = observé ; les attendus de Flux sont SUPPOSÉS.",
         "**Source retenue pour « Flux signale » et « Flux corrige l'objet » : le journal du kustomize-controller** (lecture seule ; une ligne par réconciliation, avec l'action par objet et les erreurs ; non dédupliqué, contrairement aux événements Kubernetes). Les événements et la sonde de 10 s sont gardés à côté pour comparaison.", "",
         "## Tableau par scénario", "",
         "| id | couche | source de l'erreur | SylvaTwin détecte | délai SylvaTwin | verdict conforme | fausses alertes | Flux signale : événements | Flux signale : journal (erreur) | Flux corrige l'objet : journal | Flux corrige l'objet : sonde 10 s | la conséquence disparaît (vu par SylvaTwin) | kubectl diff -k | Alertmanager | retour sain |",
         "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|"]
    for sc in order:
        rr = by[sc]; n = len(rr); r0 = rr[0]; k = lambda g: sum(1 for r in rr if g(r))
        dl = [r["delai_sylvatwin_s"] for r in rr if r.get("delai_sylvatwin_s") is not None]
        sd = (f(k(lambda r: r["sylvatwin_detecte"]), n) if r0["sylvatwin_detecte"] is not None else "sans objet")
        fo = r0.get("flux_objet")
        jc = (f(k(lambda r: r["flux_journal_corrige_objet"]), n) if fo else "—")
        sc_ = (f(k(lambda r: r["flux_corrige"]), n) if r0["flux_corrige"] is not None else "n/a (Git)")
        csq = (f(k(lambda r: r.get("resolu_avant_restauration")), n) if r0["derives_attendues"] else "—")
        kd = "n/a (Git)" if r0["git"] else f(k(lambda r: r.get("diff_kubectl_10", {}).get("non_vide")), n)
        L.append(f"| {sc} | {r0['couche']} | {r0['source']} | {sd} | {fmt_d(dl)} | {f(k(lambda r: r['verdict_conforme']), n)} | {sum(len(r['fausses_alertes']) for r in rr)} | {f(k(lambda r: r['flux_signale']), n)} | {f(k(lambda r: r['flux_journal_erreur']), n)} | {jc} | {sc_} | {csq} | {kd} | {f(k(lambda r: r['prom_detecte']), n)} | {fmt_d([r['retour_sain_s'] for r in rr if r.get('retour_sain_s') is not None])} |")
    L += ["", "« Flux corrige l'objet » = Flux a remis l'objet comme dans Git (journal : action ≠ unchanged sur l'objet visé ; sonde : objet conforme à Git). « la conséquence disparaît » = SylvaTwin n'a plus aucune dérive nouvelle avant notre restauration : **c'est la vue de SylvaTwin, pas une mesure indépendante** (cas K5 : la ConfigMap est remise par Flux, le pod n'est jamais redémarré, la dérive persiste).", ""]
    git = [sc for sc in order if load_orc(sc)["git"]]
    if git:
        L += ["## Scénarios Git : deux délais pour SylvaTwin", "", "| id | push → Flux applique | SylvaTwin depuis le push | SylvaTwin depuis l'application de Flux | référence |", "|---|---|---|---|---|"]
        for sc in git:
            rr = by[sc]
            ap = [r["flux_applique_depuis_push_s"] for r in rr if r.get("flux_applique_depuis_push_s") is not None]
            ft = [r["flux_premiere_tentative_en_echec_s"] for r in rr if r.get("flux_premiere_tentative_en_echec_s") is not None]
            dp = [r["delai_sylvatwin_depuis_push_s"] for r in rr if r.get("delai_sylvatwin_depuis_push_s") is not None]
            df = [r["delai_sylvatwin_depuis_flux_s"] for r in rr if r.get("delai_sylvatwin_depuis_flux_s") is not None]
            ref = sorted(set(r.get("flux_reference_depuis") or "—" for r in rr))
            L.append(f"| {sc} | {fmt_d(ap) if ap else ('jamais appliqué (1re tentative en échec : ' + fmt_d(ft) + ')' if ft else '—')} | {fmt_d(dp)} | {fmt_d(df)} | {', '.join(ref)} |")
        L += ["", "Délai négatif = SylvaTwin a vu la dérive avant la fin de la réconciliation de Flux (la ressource change pendant l'application).", ""]
    # rappel dans / hors modèle
    tools = [("SylvaTwin", lambda r: bool(r["sylvatwin_detecte"]), False), ("Flux signale (événement, Ready=False ou erreur du journal)", lambda r: r["flux_signale_total"], False),
             ("Flux corrige l'objet (journal)", lambda r: bool(r.get("flux_journal_corrige_objet")), False),
             ("kubectl diff -k (scénarios non Git)", lambda r: bool(r.get("diff_kubectl_10", {}).get("non_vide")), True), ("Alertmanager (règles installées)", lambda r: r["prom_detecte"], False)]
    inm = [r for r in reps if not r["benin"] and r["dans_le_modele"]]; hors = [r for r in reps if not r["benin"] and not r["dans_le_modele"]]; ben = [r for r in reps if r["benin"]]
    L += ["## Rappel : dans le modèle, hors modèle, bénins", "", "| outil | dans le modèle (F, K, N) | hors modèle (H1, H2) | détections sur bénins (B1–B4) |", "|---|---|---|---|"]
    for nm, g, nogit in tools:
        a = [r for r in inm if not (nogit and r["git"])]; b = [r for r in hors if not (nogit and r["git"])]; c = [r for r in ben if not (nogit and r["git"])]
        L.append(f"| {nm} | {f(sum(1 for r in a if g(r)), len(a))} | {f(sum(1 for r in b if g(r)), len(b))} | {'n/a (corriger n\'est pas détecter)' if 'corrige' in nm else sum(1 for r in c if g(r))} |")
    L += ["", "« SylvaTwin » = les dérives attendues sont présentes (le verdict est jugé à part, colonne « verdict conforme »). Hors modèle : SylvaTwin n'a **rien à détecter par construction** (la règle SpecDrift ne compare que `replicas`) ; la ligne ne mesure pas un défaut, elle montre ce qui échappe au modèle. Précision de SylvaTwin sur les dérives : "
          f"{sum(r['nouvelles_derives'] for r in reps) - sum(len(r['fausses_alertes']) for r in reps)}/{sum(r['nouvelles_derives'] for r in reps)} ; fausses alertes sur les répétitions : {sum(len(r['fausses_alertes']) for r in reps)}.", ""]
    # scenarios qu'aucun autre outil ne voit
    seul, corrige = [], []
    for sc in order:
        rr = by[sc]; r0 = rr[0]
        if r0["benin"] or r0["sylvatwin_detecte"] is None: continue
        sy = sum(1 for r in rr if r["sylvatwin_detecte"]) > 0
        autre = any(r["flux_signale_total"] or (not r["git"] and r.get("diff_kubectl_10", {}).get("non_vide")) or r["prom_detecte"] for r in rr)
        if sy and not autre: seul.append(sc)
        if sy and not autre and not r0["git"] and any(r.get("flux_journal_corrige_objet") or r.get("flux_corrige") for r in rr): corrige.append(sc)
    L += ["**Scénarios que SylvaTwin voit et qu'aucun autre outil ne signale** (ni événement/erreur Flux, ni `kubectl diff -k`, ni Alertmanager) : " + (", ".join(seul) or "aucun") + ".",
          "Parmi eux, ceux où Flux corrige tout de même l'objet en silence : " + (", ".join(corrige) or "aucun") + ".", ""]
    L += ["`flux diff kustomization` : non comparé (évalue la Kustomization contre le cluster de management, alors qu'elle vise un cluster distant). Analyseur statique : non comparé (aucun installé).", ""]
    dbl = [(sc, sum(1 for r in by[sc] if r["verdicts_multiples"])) for sc in order]
    L += ["## Verdicts multiples pour une même dérive (défaut des règles, voir `DIAGNOSTIC-SUSPENSION-2026-11-14.md`)", "",
          "Répétitions dont au moins une dérive a reçu deux verdicts dans le même cycle (champ enregistré à partir de la mesure v2.1 ; absent des données de la campagne v2 : voir le diagnostic pour la mesure rétrospective par le journal) : " + (", ".join(f"{a} {b}/{len(by[a])}" for a, b in dbl if b) or "aucune / non enregistré") + ".", ""]
    L += ["## Détail par répétition", ""]
    for r in sorted(reps, key=lambda r: (r["scenario"], r["rep"])):
        L.append(f"* **{r['scenario']} rép {r['rep']}** — " + ", ".join(f"{x['classe']} {'détectée en %.0f s (%s)' % (x['delai_s'], x.get('verdict_initial')) if x['detectee'] else 'NON détectée'}" + (f", final {'atteint' if x.get('verdict_final_atteint') else 'non atteint'}" + (f" après {x['verdict_final_apres_s']:.0f} s" if x.get('verdict_final_apres_s') else "") if 'verdict_final_atteint' in x else "") for x in r["derives_attendues"]) + f" ; impacts {r['impacts_observes']}/{r['impacts_attendus']} ; fausses alertes : {len(r['fausses_alertes'])}" + (" (" + "; ".join(x['classe'] + ' ' + str(x['verdict_max']) for x in r['fausses_alertes'][:4]) + ")" if r["fausses_alertes"] else "") + f" ; état sain après : {r['etat_sain_apres']}.")
    open(out + "/resultats-v2.1.md", "w").write("\n".join(L) + "\n")
    print("\n".join(L[:48]))

if __name__ == "__main__": main()
