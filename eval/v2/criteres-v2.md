# Critères et oracle v2 — SylvaTwin face à Flux CD (écrit le 14/11/2026, AVANT tout essai)

Banc neuf. Rien n'est repris de `eval/` (campagne du 09/11, caduque : TBox 1.4.0, verdict par cluster, échéance, StaleConfigDrift,
synchronisation par événements). L'oracle est dérivé des règles actuelles de `sylvatwin-poc/rules/` (derives, lifecycle, verdict) et
ne sera PAS modifié après un premier essai (si un essai échoue, on corrige le banc, pas l'oracle). Un `oracle.json` par scénario
(`scenarios/<id>/oracle.json`) est la forme exécutable de ce document.

## État de référence et bornes

* État sain : 5 StaleConfigDrift + 11 FunctionImpactDrift, tous `ConfirmedViolation` (management : rancher-turtles, 2 pods thanos ;
  workload : pods nrf et smf périmés + leurs impacts). Ces 16 dérives sont la **base** : elles ne comptent ni comme détection ni comme fausse alerte.
* Une **dérive nouvelle** = identifiant absent de la base (instantané `cycle.json` juste avant l'injection).
* **Fausse alerte** = toute dérive nouvelle qui ne correspond à aucune ligne de l'oracle du scénario (classe, ou NF d'impact hors liste), plus,
  pour un scénario bénin, toute dérive nouvelle `ConfirmedViolation`. Les dérives transitoires déclarées « tolérées » (verdict ≤ Suspected) ne sont pas fausses.
* Délai = premier cycle contenant la dérive attendue − t0 (heure du serveur, injection). Les Git (F3, N2, B3) : t0 = instant du `git push`.
* Horizons (règles lifecycle) : Kustomization active = interval 120 s + timeout 300 s = **420 s** (03) ; ConvergenceDrift = progressDeadlineSeconds 600 s (04) ;
  horizon infini (→ `ConfirmedViolation` tout de suite) pour StaleConfig, PlacementDrift, InterNF, BrokenReference, MachineNode, objets hors Git, Kustomization suspendue (05).
* Classes mères (TBox 1.4.0) : RepairedByFluxDrift (SpecDrift, GitOpsSyncDrift), RepairedByKubernetesDrift (ConvergenceDrift),
  UnattendedDrift (Placement, InterNF, BrokenReference, MachineNode, StaleConfig). FunctionImpactDrift est hors de ces classes (dérivée).
* FunctionImpact : `derives/10` (direct : Deployment/Pod de la NF ; flux : Kustomization qui gère la NF ; infrastructure : Machine/Node) puis `derives/11`
  (`:dependsOnNF+`) : au banc, **seul le NRF a des dépendants** (9 NF). InterNFConsistencyDrift ne produit aucun impact (décision du 14/11, question ouverte HANDOFF).
* « Ce que Flux est censé faire » : tout est **SUPPOSÉ** (Flux CD v2.8, Kustomization `interval 2m`, `retryInterval 1m`, `prune true`, `force false`, GitRepository `interval 2m`).

## Catalogue

| id | couche | scénario (injection) | source de l'erreur | règle | classe → mère | NF touchées (chemin) | verdict dans le temps | Flux (SUPPOSÉ) |
|---|---|---|---|---|---|---|---|---|
| K1 | L2 | `scale deploy/nrf --replicas=2` (Git : 1) | doc Flux Kustomization ; Mutiny, DSN 2024 | derives/01, 10, 11 | SpecDrift → RepairedByFlux | 10 NF : nrf (direct) + 9 (dépendance) | Suspected (horizon 420 s) ; résolue quand Flux corrige | corrige en ≤ ~2 min |
| K2 | L2 | `delete secret udm-keys` | Configuration Defects in Kubernetes, arXiv 2512.05062 | derives/04, 10 | BrokenReferenceDrift → Unattended | udm (direct) | Confirmed tout de suite (horizon infini) | ne voit rien (Secret hors Git) |
| K3 | L2 | K2 puis `delete pod` udm | idem | derives/04, 02, 10 | BrokenReference → Unattended ; ConvergenceDrift → RepairedByKubernetes | udm (direct) ×2 | BrokenRef Confirmed ; ConvergenceDrift Suspected puis Confirmed à ~600 s | ne voit rien |
| K4 | L2 | label de nœud retiré + nodeSelector | doc Kubernetes ; Mutiny | derives/03 | PlacementDrift | — | — | **RETIRÉ** (voir plus bas) |
| K5 | L2 | `amf-config` modifiée à la main, pod non redémarré | issue kubernetes/kubernetes 22368 ; cas réels du banc | derives/09, 10 | StaleConfigDrift → Unattended | amf (direct) | Confirmed tout de suite ; **persiste** même après correction de la ConfigMap par Flux | remet la ConfigMap, ne redémarre jamais le pod |
| F1 | L3 | Kustomization suspendue | observation du banc | derives/07, 10 | GitOpsSyncDrift → RepairedByFlux | 10 NF (chemin flux) | Confirmed tout de suite (suspendue = horizon infini) | ne signale rien |
| F2 | L3 | suspension puis `scale deploy/udr --replicas=2` | test 2 de la procédure | derives/07, 01, 10 | GitOpsSync + SpecDrift → RepairedByFlux | 10 NF (flux) + udr | les deux Confirmed tout de suite | ne corrige pas |
| F3 | L3 | commit Git invalide (`imagePullPolicy` inconnue sur udr) : révision non appliquée | doc Flux | derives/07, 10 | GitOpsSyncDrift → RepairedByFlux | 10 NF (flux) | Suspected puis Confirmed à 420 s | échec du dry-run, Ready=False, réessaie chaque minute |
| N1 | L4 | `mnc` 70 → 71 dans `nrf-config` (direct) | 3GPP TS 23.501 ; Open5GS | derives/05 | InterNFConsistencyDrift → Unattended | aucune (pas d'impact) | Confirmed tout de suite (horizon infini) bien que Flux corrige en ≤ 2 min | remet la ConfigMap |
| N2 | L4 | tranche sst 2 ajoutée au SMF dans Git, pas à l'AMF | idem | derives/06 | InterNFConsistencyDrift → Unattended | aucune | Confirmed ; GitOpsSync transitoire (Suspected) avant application | applique sans erreur : invisible |
| H1 | hors modèle | image de `scp` changée à la main (même image, épinglée par digest) | observation du banc | aucune (SpecDrift ne compare que replicas) | — | — | aucune dérive attendue | corrige (champ géré) |
| H2 | hors modèle | `spec.revisionHistoryLimit` de `scp` modifié (absent de Git) | observation du banc | aucune | — | — | aucune dérive attendue | ne voit rien |
| B1 | bénin | `delete pod` bsf | opération normale | 02 | ConvergenceDrift transitoire | — | Suspected au plus | — |
| B2 | bénin | `rollout restart deploy/udr` | opération normale | 02 | transitoire | — | Suspected au plus | — |
| B3 | bénin | commit Git légitime : replicas udr 1 → 2 | opération normale GitOps | 07, 01, 02 | transitoires | — | Suspected au plus, puis résolu | applique |
| B4 | bénin | `flux reconcile kustomization sylvatwin-5gc` | opération normale | — | aucune | — | aucune | — |
| C1 | L1 | Machine sans Node | issue sylva-core 1442 | derives/08 | MachineNodeMismatchDrift → Unattended | — | Confirmed (horizon infini) | — |

Verdict « conforme » = verdict initial observé = verdict initial attendu ET, quand l'oracle en donne un, verdict final observé à l'échéance dans la fenêtre.
Hors modèle (H1, H2) : comptés dans le rappel « global » mais aussi présentés à part (rappel « dans le modèle »).

## Scénarios retirés ou différés

* **K4 retiré.** Il faut un `nodeSelector` stable : une modification à la main du Deployment est annulée par Flux en ≤ 2 min, et un commit
  Git de préparation n'est autorisé que pour F3, N2 et B3. Le scénario touche aussi un nœud (le seul nœud workload). Non mesuré ; il reste couvert par la
  campagne du 09/11 (caduque pour ce banc).
* **C1 : pas d'injection réelle** (interdit : Machine/Node/cluster). Rejeu sur faits construits, marqué « rejeu, non mesuré en réel ». Voir `resultats-v2.md`.

## Outils comparés

SylvaTwin ; Flux CD (événements de la Kustomization, condition Ready, correction observée) ; `kubectl diff -k` (manifestes Git vs cluster, à t0 + 10 s) ;
`flux diff kustomization` (idem) ; Alertmanager du workload (règles Prometheus **déjà installées**, interrogé toutes les 10 s : seules les alertes nouvelles du namespace sylvatwin-5gc comptent).
Analyseur statique (kube-linter, kubeconform, etc.) : **non installé** → « non comparé ».

## Définitions des mesures

* détection k/3 : nombre de répétitions où l'outil signale (SylvaTwin : une dérive nouvelle de la classe attendue ; Flux : événement ou condition Ready=False après t0, ou correction ;
  diff : diff non vide sur la cible ; Alertmanager : alerte nouvelle) ;
* délai médian (min–max) entre t0 et le premier signal ; rappel global = détections / (répétitions de scénarios malveillants) ; précision SylvaTwin = dérives nouvelles conformes / toutes les dérives nouvelles ;
* retour à l'état sain : délai entre la fin de la restauration et l'état sain vérifié.
