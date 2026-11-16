# Résultats v2.1 — SylvaTwin face à Flux CD (PRÉLIMINAIRES)

Dossier : `/opt/nidar/sylvatwin-run/eval-v2/campagne-v2.3-10reps` ; 160 répétitions ; **mêmes données** que `resultats-v2.md` (conservé tel quel). Oracle : version 2.1 (`scenarios/*/oracle-2.1.json`, note datée du 14/11/2026 dans `criteres-v2.1.md`).
Horloge unique : serveur (UTC). t0 = injection (push Git pour F3, N2, B3 ; retrait du label pour K4). **Mesuré** = observé ; les attendus de Flux sont SUPPOSÉS.
**Source retenue pour « Flux signale » et « Flux corrige l'objet » : le journal du kustomize-controller** (lecture seule ; une ligne par réconciliation, avec l'action par objet et les erreurs ; non dédupliqué, contrairement aux événements Kubernetes). Les événements et la sonde de 10 s sont gardés à côté pour comparaison.

## Tableau par scénario

| id | couche | source de l'erreur | SylvaTwin détecte | délai SylvaTwin | verdict conforme | fausses alertes | Flux signale : événements | Flux signale : journal (erreur) | Flux corrige l'objet : journal | Flux corrige l'objet : sonde 10 s | la conséquence disparaît (vu par SylvaTwin) | kubectl diff -k | Alertmanager | retour sain |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| F1 | L3 | observation du banc | 10/10 | 4 s (4–8) | 10/10 | 0 | 0/10 | 0/10 | — | 0/10 | 0/10 | 0/10 | 0/10 | 22 s (22–22) |
| F2 | L3 | test 2 de la procedure (docs/PROCEDURE-TESTS.md) | 10/10 | 4 s (4–4) | 10/10 | 0 | 4/10 | 0/10 | 0/10 | 0/10 | 0/10 | 10/10 | 5/10 | 22 s (22–22) |
| F3 | L3 | doc Flux (dry-run / health) ; observation du banc | 10/10 | 79 s (15–129) | 10/10 | 0 | 10/10 | 10/10 | 0/10 | n/a (Git) | 0/10 | n/a (Git) | 0/10 | 80 s (43–138) |
| H1 | hors modele | observation du banc ; erreur courante de hotfix | sans objet | — | 10/10 | 0 | 6/10 | 0/10 | 10/10 | 10/10 | — | 9/10 | 3/10 | 11 s (11–11) |
| H2 | hors modele | observation du banc | sans objet | — | 10/10 | 0 | 7/10 | 0/10 | 10/10 | 10/10 | — | 0/10 | 0/10 | 11 s (11–11) |
| K1 | L2 | doc Flux Kustomization ; Mutiny, DSN 2024 | 10/10 | 6 s (6–8) | 10/10 | 0 | 2/10 | 0/10 | 10/10 | 10/10 | 10/10 | 10/10 | 0/10 | 11 s (11–11) |
| K2 | L2 | Configuration Defects in Kubernetes, arXiv 2512.05062 | 10/10 | 4 s (4–4) | 10/10 | 0 | 4/10 | 0/10 | 0/10 | 0/10 | 0/10 | 0/10 | 0/10 | 22 s (22–43) |
| K3 | L2 | Configuration Defects in Kubernetes, arXiv 2512.05062 | 10/10 | 5 s (5–6) | 10/10 | 0 | 3/10 | 0/10 | 0/10 | 0/10 | 0/10 | 0/10 | 0/10 | 32 s (32–32) |
| K4 | L2 | doc Kubernetes (nodeSelector IgnoredDuringExecution) ; Mutiny | 10/10 | 4 s (4–10) | 10/10 | 0 | 0/10 | 0/10 | — | 0/10 | 0/10 | 0/10 | 0/10 | 106 s (96–117) |
| K5 | L2 | issue kubernetes/kubernetes 22368 ; cas reels du banc | 10/10 | 4 s (4–8) | 10/10 | 0 | 4/10 | 0/10 | 10/10 | 10/10 | 0/10 | 9/10 | 0/10 | 22 s (22–22) |
| N1 | L4 | 3GPP TS 23.501, Open5GS | 9/10 | 4 s (4–6) | 9/10 | 0 | 6/10 | 0/10 | 10/10 | 10/10 | 9/10 | 9/10 | 0/10 | 11 s (11–11) |
| N2 | L4 | 3GPP TS 23.501, Open5GS | 10/10 | 77 s (48–129) | 10/10 | 0 | 0/10 | 0/10 | 10/10 | n/a (Git) | 0/10 | n/a (Git) | 0/10 | 101 s (22–138) |
| B1 | benin | operation normale | sans objet | — | 10/10 | 0 | 0/10 | 0/10 | — | 10/10 | — | 0/10 | 0/10 | 11 s (11–11) |
| B2 | benin | operation normale | sans objet | — | 10/10 | 0 | 4/10 | 0/10 | 7/10 | 10/10 | — | 0/10 | 2/10 | 11 s (11–11) |
| B3 | benin | operation normale (GitOps) | sans objet | — | 10/10 | 0 | 1/10 | 0/10 | 10/10 | n/a (Git) | — | n/a (Git) | 3/10 | 106 s (22–127) |
| B4 | benin | operation normale | sans objet | — | 10/10 | 0 | 0/10 | 0/10 | — | 10/10 | — | 0/10 | 0/10 | 11 s (11–11) |

« Flux corrige l'objet » = Flux a remis l'objet comme dans Git (journal : action ≠ unchanged sur l'objet visé ; sonde : objet conforme à Git). « la conséquence disparaît » = SylvaTwin n'a plus aucune dérive nouvelle avant notre restauration : **c'est la vue de SylvaTwin, pas une mesure indépendante** (cas K5 : la ConfigMap est remise par Flux, le pod n'est jamais redémarré, la dérive persiste).

## Scénarios Git : deux délais pour SylvaTwin

| id | push → Flux applique | SylvaTwin depuis le push | SylvaTwin depuis l'application de Flux | référence |
|---|---|---|---|---|
| F3 | jamais appliqué (1re tentative en échec : 72 s (9–123)) | 79 s (15–129) | 6 s (6–8) | premiere tentative en echec |
| N2 | 70 s (43–120) | 77 s (48–129) | 6 s (6–9) | application |
| B3 | 52 s (26–88) | — | — | application |

Délai négatif = SylvaTwin a vu la dérive avant la fin de la réconciliation de Flux (la ressource change pendant l'application).

## Rappel : dans le modèle, hors modèle, bénins

| outil | dans le modèle (F, K, N) | hors modèle (H1, H2) | détections sur bénins (B1–B4) |
|---|---|---|---|
| SylvaTwin | 99/100 | 0/20 | 0 |
| Flux signale (événement, Ready=False ou erreur du journal) | 33/100 | 13/20 | 5 |
| Flux corrige l'objet (journal) | 40/100 | 20/20 | n/a (corriger n'est pas détecter) |
| kubectl diff -k (scénarios non Git) | 38/80 | 9/20 | 0 |
| Alertmanager (règles installées) | 5/100 | 3/20 | 5 |

« SylvaTwin » = les dérives attendues sont présentes (le verdict est jugé à part, colonne « verdict conforme »). Hors modèle : SylvaTwin n'a **rien à détecter par construction** (la règle SpecDrift ne compare que `replicas`) ; la ligne ne mesure pas un défaut, elle montre ce qui échappe au modèle. Précision de SylvaTwin sur les dérives : 626/626 ; fausses alertes sur les répétitions : 0.

**Scénarios que SylvaTwin voit et qu'aucun autre outil ne signale** (ni événement/erreur Flux, ni `kubectl diff -k`, ni Alertmanager) : F1, K4, N2.
Parmi eux, ceux où Flux corrige tout de même l'objet en silence : aucun.

`flux diff kustomization` : non comparé (évalue la Kustomization contre le cluster de management, alors qu'elle vise un cluster distant). Analyseur statique : non comparé (aucun installé).

## Verdicts multiples pour une même dérive (défaut des règles, voir `DIAGNOSTIC-SUSPENSION-2026-11-14.md`)

Répétitions dont au moins une dérive a reçu deux verdicts dans le même cycle (champ enregistré à partir de la mesure v2.1 ; absent des données de la campagne v2 : voir le diagnostic pour la mesure rétrospective par le journal) : aucune / non enregistré.

## Invariants des règles (une dérive : au plus un firstDetectedAt, un verdict par cycle)

Cycles observés avec plus d'un `firstDetectedAt` pour une dérive : 0 ; cycles avec plus d'un verdict pour une dérive : 0 (sur 160 répétitions). Un compte non nul est un RÉSULTAT.

## Fenêtre de mesure et transitoires de restauration

Fenêtre de mesure d'une répétition : de t0 à l'instant juste avant le début de `restore.sh` (marque `fin_attente`), pour tous les scénarios et toutes les répétitions. Les nouvelles dérives qui n'apparaissent qu'ensuite (jusqu'au retour à l'état sain) sont comptées ici à part ; elles ne sont ni des détections ni des fausses alertes.

| id | répétitions avec transitoire | transitoires (classe, verdict max) |
|---|---|---|
| F1 | 0/10 | — |
| F2 | 0/10 | — |
| F3 | 0/10 | — |
| H1 | 0/10 | — |
| H2 | 0/10 | — |
| K1 | 0/10 | — |
| K2 | 0/10 | — |
| K3 | 0/10 | — |
| K4 | 2/10 | FunctionImpactDrift ConfirmedViolation; PlacementDrift ConfirmedViolation |
| K5 | 10/10 | ConvergenceDrift SuspectedViolation; FunctionImpactDrift SuspectedViolation |
| N1 | 0/10 | — |
| N2 | 0/10 | — |
| B1 | 0/10 | — |
| B2 | 0/10 | — |
| B3 | 1/10 | FunctionImpactDrift SuspectedViolation; SpecDrift SuspectedViolation |
| B4 | 0/10 | — |

Total : 26 dérives transitoires de restauration sur 160 répétitions.

## Détail par répétition

* **B1 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 1** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 2** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 3** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 4** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 5** — GitOpsSyncDrift détectée en 8 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 6** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 7** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 8** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 9** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 10** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 1** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 2** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 3** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 4** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 5** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 6** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 7** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 15 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 8** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 9** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 10** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 1** — GitOpsSyncDrift détectée en 129 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 2** — GitOpsSyncDrift détectée en 55 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 3** — GitOpsSyncDrift détectée en 91 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 4** — GitOpsSyncDrift détectée en 71 s (SuspectedViolation), final atteint après 420 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 5** — GitOpsSyncDrift détectée en 63 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 6** — GitOpsSyncDrift détectée en 87 s (SuspectedViolation), final atteint après 419 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 7** — GitOpsSyncDrift détectée en 53 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 8** — GitOpsSyncDrift détectée en 118 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 9** — GitOpsSyncDrift détectée en 15 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 10** — GitOpsSyncDrift détectée en 97 s (SuspectedViolation), final atteint après 421 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 3** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 4** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 5** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 6** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 7** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 8** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 9** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 10** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 1** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 2** — SpecDrift détectée en 7 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 3** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 4** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 5** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 6** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 7** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 8** — SpecDrift détectée en 8 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 9** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 10** — SpecDrift détectée en 7 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 1** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 2** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 3** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 4** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 5** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 6** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 7** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 8** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 9** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 10** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 1** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 2** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 3** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 599 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 4** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 599 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 5** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 6** — BrokenReferenceDrift détectée en 6 s (ConfirmedViolation), ConvergenceDrift détectée en 6 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 7** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 599 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 8** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 9** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 599 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 10** — BrokenReferenceDrift détectée en 5 s (ConfirmedViolation), ConvergenceDrift détectée en 5 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 1** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 2** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 3** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 4** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 5** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 6** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 7** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 8** — PlacementDrift détectée en 10 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 9** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 10** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 1** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 2** — StaleConfigDrift détectée en 8 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 3** — StaleConfigDrift détectée en 5 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 4** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 5** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 6** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 7** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 8** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 9** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 10** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 1** — InterNFConsistencyDrift NON détectée ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 2** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 3** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 4** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 5** — InterNFConsistencyDrift détectée en 5 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 6** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 7** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 8** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 9** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 10** — InterNFConsistencyDrift détectée en 6 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 1** — InterNFConsistencyDrift détectée en 75 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 2** — InterNFConsistencyDrift détectée en 68 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 3** — InterNFConsistencyDrift détectée en 70 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 4** — InterNFConsistencyDrift détectée en 124 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 5** — InterNFConsistencyDrift détectée en 68 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 6** — InterNFConsistencyDrift détectée en 79 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 7** — InterNFConsistencyDrift détectée en 101 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 8** — InterNFConsistencyDrift détectée en 96 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 9** — InterNFConsistencyDrift détectée en 129 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 10** — InterNFConsistencyDrift détectée en 48 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
