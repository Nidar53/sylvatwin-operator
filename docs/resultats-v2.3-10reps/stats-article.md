# Statistiques de la campagne (160 répétitions)

Intervalles : Wilson 95 % (en %, entre crochets). Délais en s : médiane / p95 (SylvaTwin, depuis t0 ; pour F3, N2, B3 t0 = push Git). Fenêtre de mesure : t0 → avant `restore.sh`.

| id | couche | n | SylvaTwin détecte | délai méd/p95 | verdict conforme | Flux signale | Flux corrige (journal) | kubectl diff -k | Alertmanager | transitoires de restauration |
|---|---|---|---|---|---|---|---|---|---|---|
| F1 | L3 | 10 | 10/10 [72–100] | 4 / 8 | 10/10 [72–100] | 0/10 [0–28] | — | 0/10 [0–28] | 0/10 [0–28] | 0 |
| F2 | L3 | 10 | 10/10 [72–100] | 4 / 4 | 10/10 [72–100] | 4/10 [17–69] | 0/10 [0–28] | 10/10 [72–100] | 5/10 [24–76] | 0 |
| F3 | L3 | 10 | 10/10 [72–100] | 79 / 129 | 10/10 [72–100] | 10/10 [72–100] | 0/10 [0–28] | n/a | 0/10 [0–28] | 0 |
| H1 | hors modele | 10 | sans objet | — | 10/10 [72–100] | 6/10 [31–83] | 10/10 [72–100] | 9/10 [60–98] | 3/10 [11–60] | 0 |
| H2 | hors modele | 10 | sans objet | — | 10/10 [72–100] | 7/10 [40–89] | 10/10 [72–100] | 0/10 [0–28] | 0/10 [0–28] | 0 |
| K1 | L2 | 10 | 10/10 [72–100] | 6 / 8 | 10/10 [72–100] | 2/10 [6–51] | 10/10 [72–100] | 10/10 [72–100] | 0/10 [0–28] | 0 |
| K2 | L2 | 10 | 10/10 [72–100] | 4 / 4 | 10/10 [72–100] | 4/10 [17–69] | 0/10 [0–28] | 0/10 [0–28] | 0/10 [0–28] | 0 |
| K3 | L2 | 10 | 10/10 [72–100] | 5 / 6 | 10/10 [72–100] | 3/10 [11–60] | 0/10 [0–28] | 0/10 [0–28] | 0/10 [0–28] | 0 |
| K4 | L2 | 10 | 10/10 [72–100] | 4 / 10 | 10/10 [72–100] | 0/10 [0–28] | — | 0/10 [0–28] | 0/10 [0–28] | 4 |
| K5 | L2 | 10 | 10/10 [72–100] | 4 / 8 | 10/10 [72–100] | 4/10 [17–69] | 10/10 [72–100] | 9/10 [60–98] | 0/10 [0–28] | 20 |
| N1 | L4 | 10 | 9/10 [60–98] | 4 / 6 | 9/10 [60–98] | 6/10 [31–83] | 10/10 [72–100] | 9/10 [60–98] | 0/10 [0–28] | 0 |
| N2 | L4 | 10 | 10/10 [72–100] | 77 / 129 | 10/10 [72–100] | 0/10 [0–28] | 10/10 [72–100] | n/a | 0/10 [0–28] | 0 |
| B1 | benin | 10 | sans objet | — | 10/10 [72–100] | 0/10 [0–28] | — | 0/10 [0–28] | 0/10 [0–28] | 0 |
| B2 | benin | 10 | sans objet | — | 10/10 [72–100] | 4/10 [17–69] | 7/10 [40–89] | 0/10 [0–28] | 2/10 [6–51] | 0 |
| B3 | benin | 10 | sans objet | — | 10/10 [72–100] | 1/10 [2–40] | 10/10 [72–100] | n/a | 3/10 [11–60] | 2 |
| B4 | benin | 10 | sans objet | — | 10/10 [72–100] | 0/10 [0–28] | — | 0/10 [0–28] | 0/10 [0–28] | 0 |

## Global

| ensemble | n | SylvaTwin détecte | verdict conforme | Flux signale | Flux corrige (journal) | kubectl diff -k (non Git) | Alertmanager |
|---|---|---|---|---|---|---|---|
| dans le modèle (F, K, N) | 100 | 99/100 [95–100] | 99/100 [95–100] | 33/100 [25–43] | 40/80 [39–61] | 38/80 [37–58] | 5/100 [2–11] |
| hors modèle (H1, H2) | 20 | sans objet | 20/20 [84–100] | 13/20 [43–82] | 20/20 [84–100] | 9/20 [26–66] | 3/20 [5–36] |
| bénins (B1–B4) : détections | 40 | sans objet | 40/40 [91–100] | 5/40 [5–26] | 17/20 [64–95] | 0/30 [0–11] | 5/40 [5–26] |

Délai SylvaTwin (dans le modèle, 99 mesures) : médiane 4 s, p95 101 s, max 129 s.

Fausses alertes sur la fenêtre de mesure : 0. Dérives attendues détectées avec verdict initial conforme : voir le détail par répétition de `resultats-v2.1-fenetre.md`.

## Transitoires de restauration (comptés à part)

* B3 : FunctionImpactDrift SuspectedViolation (1 occurrence(s))
* B3 : SpecDrift SuspectedViolation (1 occurrence(s))
* K4 : FunctionImpactDrift ConfirmedViolation (2 occurrence(s))
* K4 : PlacementDrift ConfirmedViolation (2 occurrence(s))
* K5 : ConvergenceDrift SuspectedViolation (10 occurrence(s))
* K5 : FunctionImpactDrift SuspectedViolation (10 occurrence(s))

## Invariants : cycles avec plus d'un firstDetectedAt : 0 ; cycles avec plus d'un verdict : 0.
