#!/usr/bin/env python3
"""Statistiques pour l'article, a partir de DOSSIER/resultats-v2.1-fenetre.json (analyse.py --version 2.1 --fenetre) :
intervalles de Wilson (95 %) par scenario et globaux, delais median et p95, transitoires de restauration, tableau LaTeX.
Usage : stats_article.py DOSSIER   -> ecrit DOSSIER/stats-article.md et DOSSIER/tableau-article.tex (lecture seule des mesures)."""
import json, math, os, sys, statistics
V2 = os.path.dirname(os.path.abspath(__file__)); D = sys.argv[1]
reps = json.load(open(D + "/resultats-v2.1-fenetre.json"))
orc = lambda s: json.load(open(f"{V2}/scenarios/{s}/oracle-2.1.json"))
def wilson(k, n, z=1.96):
    if n == 0: return (0.0, 0.0)
    p = k / n; d = 1 + z * z / n; c = (p + z * z / (2 * n)) / d; h = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n)) / d
    return (max(0.0, c - h), min(1.0, c + h))
def pct(k, n): lo, hi = wilson(k, n); return f"{k}/{n} [{100*lo:.0f}–{100*hi:.0f}]"
def p95(v):
    v = sorted(v); return v[min(len(v) - 1, math.ceil(0.95 * len(v)) - 1)] if v else None
def dl(v): return "—" if not v else f"{statistics.median(v):.0f} / {p95(v):.0f}"
by = {}
for r in reps: by.setdefault(r["scenario"], []).append(r)
order = sorted(by, key=lambda s: (orc(s)["couche"] == "benin", s))
T = {  # outil -> predicat sur une repetition ; None = non applicable
 "sylvatwin": lambda r: bool(r["sylvatwin_detecte"]),
 "verdict": lambda r: bool(r["verdict_conforme"]),
 "flux_signale": lambda r: bool(r["flux_signale_total"]),
 "flux_corrige": lambda r: bool(r.get("flux_journal_corrige_objet")),
 "kdiff": lambda r: bool(r.get("diff_kubectl_10", {}).get("non_vide")),
 "prom": lambda r: bool(r["prom_detecte"])}
L = [f"# Statistiques de la campagne ({len(reps)} répétitions)", "", "Intervalles : Wilson 95 % (en %, entre crochets). Délais en s : médiane / p95 (SylvaTwin, depuis t0 ; pour F3, N2, B3 t0 = push Git). Fenêtre de mesure : t0 → avant `restore.sh`.", "",
     "| id | couche | n | SylvaTwin détecte | délai méd/p95 | verdict conforme | Flux signale | Flux corrige (journal) | kubectl diff -k | Alertmanager | transitoires de restauration |", "|---|---|---|---|---|---|---|---|---|---|---|"]
tex = []
cell = lambda sc, key, rr: ("n/a" if (key == "kdiff" and orc(sc)["git"]) else pct(sum(1 for r in rr if T[key](r)), len(rr)))
for sc in order:
    rr = by[sc]; o = orc(sc); n = len(rr)
    sy = "sans objet" if rr[0]["sylvatwin_detecte"] is None else cell(sc, "sylvatwin", rr)
    dd = [r["delai_sylvatwin_s"] for r in rr if r.get("delai_sylvatwin_s") is not None]
    tr = sum(len(r.get("transitoires_restauration", [])) for r in rr)
    fc = "—" if not o.get("flux_objet") else cell(sc, "flux_corrige", rr)
    L.append(f"| {sc} | {o['couche']} | {n} | {sy} | {dl(dd)} | {cell(sc, 'verdict', rr)} | {cell(sc, 'flux_signale', rr)} | {fc} | {cell(sc, 'kdiff', rr)} | {cell(sc, 'prom', rr)} | {tr} |")
    f = lambda k, nn: f"{k}/{nn}"
    esc = lambda s: s.replace("_", r"\_").replace("&", r"\&").replace("%", r"\%").replace("#", r"\#")
    tex.append(" & ".join([sc, esc(o["couche"]), esc(o["titre"]), esc(o["source_erreur"].split(" ; ")[0]),
        "n/a" if rr[0]["sylvatwin_detecte"] is None else f(sum(1 for r in rr if T["sylvatwin"](r)), n), ("--" if not dd else f"{statistics.median(dd):.0f}"),
        f(sum(1 for r in rr if T["verdict"](r)), n), f(sum(1 for r in rr if T["flux_signale"](r)), n),
        "--" if not o.get("flux_objet") else f(sum(1 for r in rr if T["flux_corrige"](r)), n),
        "n/a" if o["git"] else f(sum(1 for r in rr if T["kdiff"](r)), n), f(sum(1 for r in rr if T["prom"](r)), n)]) + r" \\")
g = lambda sel: [r for r in reps if sel(orc(r["scenario"]))]
inm = g(lambda o: not o["benin"] and o["dans_le_modele"]); hors = g(lambda o: not o["benin"] and not o["dans_le_modele"]); ben = g(lambda o: o["benin"])
L += ["", "## Global", "", "| ensemble | n | SylvaTwin détecte | verdict conforme | Flux signale | Flux corrige (journal) | kubectl diff -k (non Git) | Alertmanager |", "|---|---|---|---|---|---|---|---|"]
for nm, rr in (("dans le modèle (F, K, N)", inm), ("hors modèle (H1, H2)", hors), ("bénins (B1–B4) : détections", ben)):
    ng = [r for r in rr if not r["git"]]
    row = [nm, str(len(rr))]
    row.append("sans objet" if nm.startswith(("hors", "béni")) else pct(sum(1 for r in rr if T["sylvatwin"](r)), len(rr)))
    row.append(pct(sum(1 for r in rr if T["verdict"](r)), len(rr)))
    row.append(pct(sum(1 for r in rr if T["flux_signale"](r)), len(rr)))
    fo = [r for r in rr if orc(r["scenario"]).get("flux_objet")]
    row.append(pct(sum(1 for r in fo if T["flux_corrige"](r)), len(fo)) if fo else "—")
    row.append(pct(sum(1 for r in ng if T["kdiff"](r)), len(ng)) if ng else "—")
    row.append(pct(sum(1 for r in rr if T["prom"](r)), len(rr)))
    L.append("| " + " | ".join(row) + " |")
dd = [r["delai_sylvatwin_s"] for r in inm if r.get("delai_sylvatwin_s") is not None]
L += ["", f"Délai SylvaTwin (dans le modèle, {len(dd)} mesures) : médiane {statistics.median(dd):.0f} s, p95 {p95(dd):.0f} s, max {max(dd):.0f} s." if dd else "",
      "", "Fausses alertes sur la fenêtre de mesure : %d. Dérives attendues détectées avec verdict initial conforme : voir le détail par répétition de `resultats-v2.1-fenetre.md`." % sum(len(r["fausses_alertes"]) for r in reps), "",
      "## Transitoires de restauration (comptés à part)", ""]
tr = {}
for r in reps:
    for t in r.get("transitoires_restauration", []): tr.setdefault((r["scenario"], t["classe"], t["verdict_max"]), 0); tr[(r["scenario"], t["classe"], t["verdict_max"])] += 1
L += [f"* {a} : {c} {b} ({n} occurrence(s))" for (a, c, b), n in sorted(tr.items())] or ["Aucun."]
inv = (sum(r["cycles_fd_multiples"] for r in reps), sum(r["cycles_verdicts_multiples"] for r in reps))
L += ["", f"## Invariants : cycles avec plus d'un firstDetectedAt : {inv[0]} ; cycles avec plus d'un verdict : {inv[1]}."]
open(D + "/stats-article.md", "w").write("\n".join(L) + "\n")
hdr = [r"\begin{table*}[t]", r"\centering", r"\caption{Detection par scenario (oracle 2.1). Valeurs : repetitions conformes / repetitions ; delai = mediane en s (t0 = injection ou push Git). n/a : non applicable.}",
       r"\label{tab:resultats}", r"\small", r"\begin{tabular}{llp{3.6cm}p{2.4cm}rrrrrrr}", r"\hline",
       r"Id & Couche & Erreur & Source & SylvaTwin & D\'elai & Verdict & Flux signale & Flux corrige & kubectl diff & Alertmanager \\", r"\hline"]
open(D + "/tableau-article.tex", "w").write("\n".join(hdr + tex + [r"\hline", r"\end{tabular}", r"\end{table*}"]) + "\n")
print("ecrit", D + "/stats-article.md", D + "/tableau-article.tex")
