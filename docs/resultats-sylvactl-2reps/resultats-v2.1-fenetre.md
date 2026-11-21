# Résultats v2.1 — SylvaTwin face à Flux CD (PRÉLIMINAIRES)

Dossier : `/opt/nidar/sylvatwin-run/eval-v2/campagne-sylvactl-2reps` ; 32 répétitions ; **mêmes données** que `resultats-v2.md` (conservé tel quel). Oracle : version 2.1 (`scenarios/*/oracle-2.1.json`, note datée du 14/11/2026 dans `criteres-v2.1.md`).
Horloge unique : serveur (UTC). t0 = injection (push Git pour F3, N2, B3 ; retrait du label pour K4). **Mesuré** = observé ; les attendus de Flux sont SUPPOSÉS.
**Source retenue pour « Flux signale » et « Flux corrige l'objet » : le journal du kustomize-controller** (lecture seule ; une ligne par réconciliation, avec l'action par objet et les erreurs ; non dédupliqué, contrairement aux événements Kubernetes). Les événements et la sonde de 10 s sont gardés à côté pour comparaison.

## Tableau par scénario

| id | couche | source de l'erreur | SylvaTwin détecte | délai SylvaTwin | verdict conforme | fausses alertes | Flux signale : événements | Flux signale : journal (erreur) | Flux corrige l'objet : journal | Flux corrige l'objet : sonde 10 s | la conséquence disparaît (vu par SylvaTwin) | kubectl diff -k | Alertmanager | retour sain |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| F1 | L3 | observation du banc | 2/2 | 4 s (4–4) | 2/2 | 0 | 0/2 | 0/2 | — | 0/2 | 0/2 | 0/2 | 0/2 | 22 s (22–22) |
| F2 | L3 | test 2 de la procedure (docs/PROCEDURE-TESTS.md) | 2/2 | 4 s (4–4) | 2/2 | 0 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 2/2 | 0/2 | 22 s (22–22) |
| F3 | L3 | doc Flux (dry-run / health) ; observation du banc | 2/2 | 119 s (109–130) | 2/2 | 0 | 2/2 | 2/2 | 0/2 | n/a (Git) | 0/2 | n/a (Git) | 0/2 | 59 s (43–74) |
| H1 | hors modele | observation du banc ; erreur courante de hotfix | sans objet | — | 2/2 | 0 | 1/2 | 0/2 | 2/2 | 2/2 | — | 2/2 | 1/2 | 11 s (11–11) |
| H2 | hors modele | observation du banc | sans objet | — | 2/2 | 0 | 1/2 | 0/2 | 2/2 | 2/2 | — | 0/2 | 0/2 | 11 s (11–11) |
| K1 | L2 | doc Flux Kustomization ; Mutiny, DSN 2024 | 2/2 | 6 s (6–6) | 2/2 | 0 | 2/2 | 0/2 | 2/2 | 2/2 | 2/2 | 2/2 | 0/2 | 11 s (11–11) |
| K2 | L2 | Configuration Defects in Kubernetes, arXiv 2512.05062 | 2/2 | 3 s (1–4) | 2/2 | 0 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 22 s (22–22) |
| K3 | L2 | Configuration Defects in Kubernetes, arXiv 2512.05062 | 2/2 | 6 s (6–6) | 2/2 | 0 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 0/2 | 32 s (32–32) |
| K4 | L2 | doc Kubernetes (nodeSelector IgnoredDuringExecution) ; Mutiny | 2/2 | 5 s (4–5) | 2/2 | 0 | 0/2 | 0/2 | — | 0/2 | 0/2 | 0/2 | 0/2 | 111 s (106–117) |
| K5 | L2 | issue kubernetes/kubernetes 22368 ; cas reels du banc | 2/2 | 4 s (4–5) | 2/2 | 0 | 1/2 | 0/2 | 2/2 | 2/2 | 0/2 | 2/2 | 0/2 | 22 s (22–22) |
| N1 | L4 | 3GPP TS 23.501, Open5GS | 2/2 | 4 s (4–4) | 2/2 | 0 | 0/2 | 0/2 | 2/2 | 2/2 | 2/2 | 2/2 | 0/2 | 11 s (11–11) |
| N2 | L4 | 3GPP TS 23.501, Open5GS | 2/2 | 69 s (64–75) | 2/2 | 0 | 0/2 | 0/2 | 2/2 | n/a (Git) | 0/2 | n/a (Git) | 0/2 | 138 s (127–148) |
| B1 | benin | operation normale | sans objet | — | 2/2 | 0 | 0/2 | 0/2 | — | 2/2 | — | 0/2 | 0/2 | 11 s (11–11) |
| B2 | benin | operation normale | sans objet | — | 2/2 | 0 | 1/2 | 0/2 | 2/2 | 2/2 | — | 0/2 | 1/2 | 11 s (11–11) |
| B3 | benin | operation normale (GitOps) | sans objet | — | 2/2 | 0 | 0/2 | 0/2 | 2/2 | n/a (Git) | — | n/a (Git) | 0/2 | 37 s (22–53) |
| B4 | benin | operation normale | sans objet | — | 2/2 | 0 | 0/2 | 0/2 | — | 2/2 | — | 0/2 | 0/2 | 11 s (11–11) |

« Flux corrige l'objet » = Flux a remis l'objet comme dans Git (journal : action ≠ unchanged sur l'objet visé ; sonde : objet conforme à Git). « la conséquence disparaît » = SylvaTwin n'a plus aucune dérive nouvelle avant notre restauration : **c'est la vue de SylvaTwin, pas une mesure indépendante** (cas K5 : la ConfigMap est remise par Flux, le pod n'est jamais redémarré, la dérive persiste).

## Scénarios Git : deux délais pour SylvaTwin

| id | push → Flux applique | SylvaTwin depuis le push | SylvaTwin depuis l'application de Flux | référence |
|---|---|---|---|---|
| F3 | jamais appliqué (1re tentative en échec : 114 s (103–124)) | 119 s (109–130) | 6 s (5–6) | premiere tentative en echec |
| N2 | 64 s (59–69) | 69 s (64–75) | 6 s (5–6) | application |
| B3 | 91 s (76–105) | — | — | application |

Délai négatif = SylvaTwin a vu la dérive avant la fin de la réconciliation de Flux (la ressource change pendant l'application).

## Rappel : dans le modèle, hors modèle, bénins

| outil | dans le modèle (F, K, N) | hors modèle (H1, H2) | détections sur bénins (B1–B4) |
|---|---|---|---|
| SylvaTwin | 20/20 | 0/4 | 0 |
| Flux signale (événement, Ready=False ou erreur du journal) | 5/20 | 2/4 | 1 |
| Flux corrige l'objet (journal) | 8/20 | 4/4 | n/a (corriger n'est pas détecter) |
| kubectl diff -k (scénarios non Git) | 8/16 | 2/4 | 0 |
| Alertmanager (règles installées) | 0/20 | 1/4 | 1 |

« SylvaTwin » = les dérives attendues sont présentes (le verdict est jugé à part, colonne « verdict conforme »). Hors modèle : SylvaTwin n'a **rien à détecter par construction** (la règle SpecDrift ne compare que `replicas`) ; la ligne ne mesure pas un défaut, elle montre ce qui échappe au modèle. Précision de SylvaTwin sur les dérives : 124/124 ; fausses alertes sur les répétitions : 0.

**Scénarios que SylvaTwin voit et qu'aucun autre outil ne signale** (ni événement/erreur Flux, ni `kubectl diff -k`, ni Alertmanager) : F1, K2, K3, K4, N2.
Parmi eux, ceux où Flux corrige tout de même l'objet en silence : aucun.

`flux diff kustomization` : non comparé (évalue la Kustomization contre le cluster de management, alors qu'elle vise un cluster distant). Analyseur statique : non comparé (aucun installé).

## Verdicts multiples pour une même dérive (défaut des règles, voir `DIAGNOSTIC-SUSPENSION-2026-11-14.md`)

Répétitions dont au moins une dérive a reçu deux verdicts dans le même cycle (champ enregistré à partir de la mesure v2.1 ; absent des données de la campagne v2 : voir le diagnostic pour la mesure rétrospective par le journal) : aucune / non enregistré.

## Invariants des règles (une dérive : au plus un firstDetectedAt, un verdict par cycle)

Cycles observés avec plus d'un `firstDetectedAt` pour une dérive : 0 ; cycles avec plus d'un verdict pour une dérive : 0 (sur 32 répétitions). Un compte non nul est un RÉSULTAT.

## Fenêtre de mesure et transitoires de restauration

Fenêtre de mesure d'une répétition : de t0 à l'instant juste avant le début de `restore.sh` (marque `fin_attente`), pour tous les scénarios et toutes les répétitions. Les nouvelles dérives qui n'apparaissent qu'ensuite (jusqu'au retour à l'état sain) sont comptées ici à part ; elles ne sont ni des détections ni des fausses alertes.

| id | répétitions avec transitoire | transitoires (classe, verdict max) |
|---|---|---|
| F1 | 0/2 | — |
| F2 | 0/2 | — |
| F3 | 0/2 | — |
| H1 | 0/2 | — |
| H2 | 0/2 | — |
| K1 | 0/2 | — |
| K2 | 0/2 | — |
| K3 | 0/2 | — |
| K4 | 0/2 | — |
| K5 | 2/2 | ConvergenceDrift SuspectedViolation; FunctionImpactDrift SuspectedViolation |
| N1 | 0/2 | — |
| N2 | 0/2 | — |
| B1 | 0/2 | — |
| B2 | 0/2 | — |
| B3 | 0/2 | — |
| B4 | 0/2 | — |

Total : 4 dérives transitoires de restauration sur 32 répétitions.

## Détail par répétition

* **B1 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B1 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B2 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B3 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **B4 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 1** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F1 rép 2** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 1** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 14 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F2 rép 2** — GitOpsSyncDrift détectée en 4 s (ConfirmedViolation), SpecDrift détectée en 10 s (ConfirmedViolation) ; impacts 11/11 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 1** — GitOpsSyncDrift détectée en 130 s (SuspectedViolation), final atteint après 422 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **F3 rép 2** — GitOpsSyncDrift détectée en 109 s (SuspectedViolation), final atteint après 419 s ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H1 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 1** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **H2 rép 2** —  ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 1** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K1 rép 2** — SpecDrift détectée en 6 s (SuspectedViolation) ; impacts 10/10 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 1** — BrokenReferenceDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K2 rép 2** — BrokenReferenceDrift détectée en 1 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 1** — BrokenReferenceDrift détectée en 6 s (ConfirmedViolation), ConvergenceDrift détectée en 6 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K3 rép 2** — BrokenReferenceDrift détectée en 6 s (ConfirmedViolation), ConvergenceDrift détectée en 6 s (SuspectedViolation), final atteint après 600 s ; impacts 2/2 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 1** — PlacementDrift détectée en 5 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K4 rép 2** — PlacementDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 1** — StaleConfigDrift détectée en 4 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **K5 rép 2** — StaleConfigDrift détectée en 5 s (ConfirmedViolation) ; impacts 1/1 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 1** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N1 rép 2** — InterNFConsistencyDrift détectée en 4 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 1** — InterNFConsistencyDrift détectée en 64 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
* **N2 rép 2** — InterNFConsistencyDrift détectée en 75 s (ConfirmedViolation) ; impacts 0/0 ; fausses alertes : 0 ; état sain après : True.
