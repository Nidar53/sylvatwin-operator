# Résultats de la campagne du 09/11/2026 (14:26 → 16:32 UTC)

**Versions utilisées**
- critères et oracle : `eval/criteres.md`, commit `b00cf2b`, fixés avant la campagne ;
- opérateur : `32b5648` ;
- règles et ontologie : sylvatwin-poc `ce6ecc7`.

**Données brutes**
- `eval/runs/<scénario>/<rép>/` : journal, injection, observation Flux, kubectl et events, puis `cycle.json`, `drifts.nt` et `verdicts.nt` de chaque cycle ;
- `eval/resultats.json` et `eval/resultats-bruts.txt` : produits par `eval/scripts/analyse.py` ;
- les faits de chaque cycle (`facts.*`, 872 fichiers, 733 Mo) sont archivés hors Git : `/opt/nidar/sylvatwin-run/facts-campagne-20261109.tar.gz`.

**Conditions de mesure**
- boucle `sylvatwin-once` : pause de 15 s entre deux cycles ;
- durée d'un cycle : médiane 2,9 s, p95 4,1 s (436 cycles) ;
- Flux : `sylvatwin-5gc` avec interval 2 min, timeout 5 min.

## Tableau principal (3 répétitions par ligne)

Notes sur les colonnes :
- **Délai** : médiane (minimum–maximum) entre l'injection et le premier cycle qui contient la dérive.
- **« Flux voit »** : statut, event ou log qui désigne l'objet ou l'écart.
- **Correction Flux** : observée par sondage toutes les 10 s environ.

| Scénario | Erreur injectée | Flux voit ? | Flux corrige ? (délai) | kubectl montre ? | SylvaTwin détecte ? (délai) | Dérives produites | NF touchées (chemin) | Verdict | Ce que SylvaTwin explique en plus |
|---|---|---|---|---|---|---|---|---|---|
| S0-A | `scale deploy/nrf --replicas=0` | Oui, **après coup** : event `Deployment/nrf configured` à la réconciliation suivante | Oui (31 / 83 / 83 s) | `get` : 0/0 pendant la dérive ; `describe` : events de scaling ; `diff -k` : la dérive, avant correction | 3/3 (14,4 s ; 14,3–14,8) | SpecDrift + 9 FunctionImpactDrift | nrf (direct) ; amf, ausf, bsf, nssf, pcf, smf, udm, udr (dépendance) | Suspectée → résolue | Les 8 NF qui dépendent du NRF ; l'écart avant correction ; « attendue » au lieu d'alerte |
| S3-b | idem (angle NF) | idem | Oui (41 / 93 / 93 s) | idem | 3/3 (14,7 s ; 14,4–14,8) | idem | idem | Suspectée → résolue | idem |
| S0-B | Suppression de `udm-keys`, puis du pod udm : le nouveau pod reste en `Init` | **Non** : Kustomization `Ready=True` tout du long | Non (pod bloqué 15/15 sondages) | Oui, en creusant : `describe pod` affiche `FailedMount secret "udm-keys" not found` ; `diff -k` vide | 3/3 (14,6 s ; 14,5–14,7) | ConvergenceDrift + BrokenReferenceDrift + 2 FunctionImpactDrift | udm (direct) | Convergence : **suspectée** ; BrokenReference : confirmée | Relie la non-convergence à sa cause (Secret absent) et à la NF |
| S2 | `delete secret udm-keys` | **Non** | Non (Secret hors Git) | Pod udm `Running 1/1` ; `get secret` → NotFound ; `diff -k` vide | 3/3 (14,2 s ; 14,0–14,5) | BrokenReferenceDrift + FunctionImpactDrift | udm (direct) | Confirmée | Dérive invisible tant que le pod ne redémarre pas |
| S3-a | idem (angle NF) | Non | Non | idem | 3/3 (14,4 s ; 14,3–14,4) | idem | udm seulement | Confirmée | idem |
| S1 | Label `zone=a` retiré du nœud, alors que bsf exige `zone=a` (commit Git de test) | **Non** | Non | Pod bsf `Running` sur le nœud ; `nodeSelector` visible ; `diff -k` vide | 3/3 (14,7 s ; 14,1–14,7) | PlacementDrift + FunctionImpactDrift | bsf seulement | Confirmée | Placement devenu invalide alors que Kubernetes n'évince pas |
| S3-c | idem (angle NF) | Non | Non | idem | 3/3 (14,8 s ; 14,5–14,8) | idem | bsf seulement (les autres NF du nœud ne sont pas touchées) | Confirmée | idem |
| S4-a | `nrf-config` modifiée en direct (mnc 70 → 71) | Oui, **après coup** : `ConfigMap/nrf-config configured` | Oui, la ConfigMap (93 / 94 / 94 s) ; le pod NRF n'est **pas** redémarré | `diff -k` : la dérive, avant correction | 3/3 (14,8 s ; 14,8–14,9) | 2 InterNFConsistencyDrift (nrf/amf, nrf/smf) | nrf, amf, smf (`affectsFunction`) | Confirmée → résolue quand Flux restaure | L'incohérence **entre NF**, pas seulement l'objet modifié |
| S4-b | `smf.yaml` dans Git : `sst: 2` ajouté, pas dans l'AMF (commit de test) | Oui, mais comme un **changement normal** : `smf-config configured` | Non : c'est conforme à Git | `diff -k` vide (conforme à Git) | 3/3 (14,4 s ; 14,4–14,7) | InterNFConsistencyDrift (tranche) | smf | Confirmée | Défaut de spécification que tous les outils jugent conforme |
| S5 | `flux suspend kustomization sylvatwin-5gc` | Oui : `SUSPENDED True`, log « Reconciliation is suspended » ; `READY` reste `True` | Non (par définition) | `flux get` : SUSPENDED=True | 3/3 (14,6 s ; 14,2–14,7) | GitOpsSyncDrift + 9 FunctionImpactDrift | les 9 NF modélisées (flux) | Confirmée | Quelles NF ne sont plus pilotées par Git |

## Événements bénins (faux positifs)

| Événement | Répétitions | Dérives | Suspectées | Confirmées | Remarque |
|---|---|---|---|---|---|
| B1 : `delete pod` ausf (recréé seul) | 3 | 0 | 0 | 0 | Le nouveau pod était prêt en moins de 10 s, avant le cycle suivant. Aucun cycle n'a vu `actualReplicas ≠ 1`. |
| B2 : `rollout restart deploy/pcf` | 3 | 0 | 0 | 0 | Mise à jour progressive (maxUnavailable 25 % arrondi à 0) : `readyReplicas` est resté à 1 tout du long. |

**Ce que ces 0 prouvent, et ce qu'ils ne prouvent pas.** Avec une période d'environ 18 s et des NF qui redémarrent en moins de 10 s, la boucle n'a jamais vu l'état passager. Le zéro faux positif tient donc **en partie à l'échantillonnage**, et pas seulement aux règles. Ce n'est pas une preuve de robustesse aux états passagers.

## Critères (fixés avant la campagne)

| # | Critère | Seuil | Résultat | Atteint ? |
|---|---|---|---|---|
| C1 | Détection de bout en bout | 10/10 scénarios, 3/3 répétitions | 10/10, 30/30 | **oui** |
| C2 | Délai médian ≤ période + 2 × cycle (≈ 20 s) | ≥ 9/10 | 10/10 (14,2 à 14,8 s) | **oui** |
| C3 | Scénarios non vus ou non expliqués par Flux **et** kubectl | N ≥ 4 | N = 7 : S0-B, S2, S3-a, S1, S3-c, S4-b, S5 | **oui** |
| C4 | Impacts inter-couches conformes à l'oracle (S3-a, S3-b, S3-c) | 3/3 à chaque répétition | 9/9 | **oui** |
| C5 | Zéro faux positif confirmé (bénins + état sain avant chaque scénario) | 0 | 0 confirmée, 0 suspectée ; 36 cycles témoins sans dérive | **oui** (portée limitée, voir ci-dessus) |
| C6 | Verdicts conformes à l'oracle | 10/10 | 10/10 (S0-B conforme à l'oracle, qui prévoyait déjà la limite) | **oui** |
| C7 | Reproductibilité depuis GitHub | oui | `go test ./...` OK ; scripts, journaux et analyse dans `eval/` | **oui** |
| C8 | Retour à l'état sain | 100 % | 36/36 répétitions + préparation S1 défaite | **oui** |

**Comment N = 7 a été compté pour C3.**
- **Comptés :**
  - S0-B : Flux ne voit rien. kubectl montre la cause seulement si l'on décrit le bon pod, et pas l'impact sur la NF.
  - S2, S3-a, S1, S3-c : ni Flux, ni `diff`, ni `get` ne montrent un écart.
  - S4-b : tous les outils jugent l'état conforme.
  - S5 : Flux affiche la suspension, mais pas les NF concernées.
- **Non comptés :** S0-A, S3-b et S4-a, que Flux voit et corrige.

## Discussion des critères (séparée, sans changer le verdict)

- **C5 était trop facile à atteindre sur ce banc.** Il aurait fallu un bénin qui reste visible au moins un cycle, par exemple un pod qui met plus de 30 s à devenir prêt. Sinon, la règle de convergence n'est pas réellement mise à l'épreuve.
- **C6 accepte une limite connue.** S0-B donne une `ConvergenceDrift` suspectée, avec l'horizon de Flux, alors que Flux ne la corrigera pas. Le verdict est conforme à l'oracle, mais pas juste sur le fond : l'horizon pertinent est le `progressDeadlineSeconds` du Deployment.
- **C2 mesure surtout la période de la boucle**, pas la capacité de l'architecture. Le délai d'environ 14,5 s correspond à « attendre le prochain cycle ».
- **C3 pour S5 se discute.** Flux montre bien la suspension. L'apport tient seulement à l'impact sur les NF.

## Observation de coût, faite après la campagne

Après 436 cycles, la base Oxigraph occupait **5,7 Go**, alors que les graphes ne contenaient que 5 535 triplets. Chaque cycle remplace intégralement le graphe des faits (PUT) et réécrit le graphe par défaut à chaque passe : RocksDB accumule alors des fichiers de journal sans les compacter. La base a été vidée après la campagne (11 Mo après un cycle).

**À traiter avant la phase 3** : écritures incrémentales, compaction, ou base en mémoire (`oxigraph serve` sans `--location`). Une boucle continue à 15 s rendrait sinon le disque saturé.
