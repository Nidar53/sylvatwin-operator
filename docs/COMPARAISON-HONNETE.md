# Comparaison honnête : SylvaTwin, Flux CD, sylvactl, kubectl diff, Alertmanager (21/11/2026)

Sources (lecture seule, aucun résultat existant modifié) :

* SylvaTwin, Flux, `kubectl diff -k`, Alertmanager : **campagne à 10 répétitions** (`campagne-v2.3-10reps`, 15–16/11/2026, 160 répétitions,
  oracle 2.1, fenêtre de mesure fermée avant `restore.sh`), avec les définitions fixées le 20/11 (`docs/VERIFICATIONS-PARTIE-A-2026-11-20.md`).
* sylvactl : **petite campagne à 2 répétitions** (`campagne-sylvactl-2reps`, 20/11/2026, 32 répétitions), attendus écrits avant l'essai
  (`eval/v2/criteres-sylvactl.md`). **k/2, sans intervalle de confiance.** C'est une autre campagne que celle des 10 répétitions : mêmes scripts
  d'injection et de restauration, autres jours.
* Contrôle de cohérence (VÉRIFIÉ) : dans la campagne sylvactl, SylvaTwin est conforme à l'oracle sur 32/32 répétitions, 0 fausse alerte,
  0 invariant violé ; les deux campagnes se sont terminées seules, état sain et `./usecase/5gc` identique au commit de départ `22c70e6`.

## 1. Tableau par scénario

« SylvaTwin » = dérive attendue présente **et** verdict conforme (k/10). « Flux nomme » = une ligne de Flux dans la fenêtre cite l'objet injecté et
l'écart en cours. « Flux corrige un écart » = Flux ré-applique l'objet à la même révision (pas l'application d'un commit). « — » = pas d'objet pour
cet outil ; « n/a » = mesure sans objet (scénario Git pour `kubectl diff`, application d'un commit pour « corrige un écart »).

| id | couche | erreur injectée | SylvaTwin (k/10) | Flux nomme (k/10) | Flux corrige un écart (k/10) | sylvactl (k/2) | kubectl diff -k (k/10) | Alertmanager (k/10) |
|---|---|---|---|---|---|---|---|---|
| F1 | L3 | Kustomization suspendue | 10/10 | 0/10 | — | **2/2** | 0/10 | 0/10 |
| F2 | L3 | suspension puis scale udr | 10/10 | 0/10 | 0/10 | **2/2** | 10/10 | 5/10 |
| F3 | L3 | commit Git invalide | 10/10 | 10/10 | n/a (commit rejeté : 0/10 appliqué) | **2/2** | n/a | 0/10 |
| K1 | L2 | scale nrf 1 → 2 | 10/10 | 2/10 | 10/10 | 0/2 | 10/10 | 0/10 |
| K2 | L2 | Secret udm-keys supprimé | 10/10 | 0/10 | 0/10 | 0/2 | 0/10 | 0/10 |
| K3 | L2 | Secret supprimé puis pod udm | 10/10 | 0/10 | 0/10 | 0/2 | 0/10 | 0/10 |
| K4 | L2 | label de nœud retiré (nodeSelector) | 10/10 | 0/10 | — | 0/2 | 0/10 | 0/10 |
| K5 | L2 | ConfigMap amf modifiée, pod non redémarré | 10/10 | 4/10 | 10/10 (la StaleConfigDrift persiste après : 10/10) | 0/2 | 9/10 | 0/10 |
| N1 | L4 | PLMN changé dans nrf-config | **9/10** | 6/10 | 10/10 | 0/2 | 9/10 | 0/10 |
| N2 | L4 | tranche ajoutée au SMF dans Git | 10/10 | 0/10 | n/a (commit appliqué : 10/10) | 0/2 | n/a | 0/10 |
| H1 | hors modèle | image changée (même digest) | 0/10 (sans objet) | 5/10 | 10/10 | 0/2 | 9/10 | 3/10 |
| H2 | hors modèle | revisionHistoryLimit modifié | 0/10 (sans objet) | 7/10 | 10/10 (valeur remise : SUPPOSÉ) | 0/2 | 0/10 | 0/10 |
| B1 | bénin | pod supprimé | 0 fausse alerte | 0/10 | — | 0/2 | 0/10 | 0/10 |
| B2 | bénin | rollout restart | 0 fausse alerte | 4/10 | 7/10 | 0/2 | 0/10 | 2/10 |
| B3 | bénin | commit Git légitime | 0 fausse alerte | 0/10 | n/a (commit appliqué : 10/10) | 0/2 | n/a | 3/10 |
| B4 | bénin | flux reconcile | 0 fausse alerte | 0/10 | — | 0/2 | 0/10 | 0/10 |

Tous les chiffres du tableau sont VÉRIFIÉS (fichiers de mesure). Totaux dans le modèle (F, K, N) : SylvaTwin **99/100**, Flux nomme 22/100,
Flux corrige un écart 30/60, sylvactl 6/20 (F1, F2, F3 seulement), `kubectl diff -k` 38/80, Alertmanager 5/100. Bénins : SylvaTwin 0 fausse
alerte sur 40 (VÉRIFIÉ) ; Flux nomme 4/40 (B2) ; Alertmanager 5/40 ; sylvactl 0/8.

### sylvactl : notre Kustomization et les autres unités de Sylva (séparés)

* **Sur notre Kustomization `sylvatwin-flux/sylvatwin-5gc`** (VÉRIFIÉ) : F1 et F2 2/2, à t0 + 7–8 s, message `Kustomization/sylvatwin-flux/sylvatwin-5gc Suspended` ;
  F3 2/2, à t0 + 107 et 127 s (après la première tentative de Flux), message `ReconciliationFailed - Deployment/sylvatwin-5gc/udr dry-run failed (Invalid): …
  imagePullPolicy: Unsupported value: "Bogus"` ; retour à « All Done » 8–9 s (F1, F2) et 26–56 s (F3) après la restauration. Aucun autre scénario.
  Les trois correspondent exactement aux attendus écrits avant l'essai.
* **Sur d'autres unités de Sylva** (VÉRIFIÉ) : dans 6 répétitions (B4, H1, K3, K4, K5, N2, une chacune ; 9 relevés en tout dans la fenêtre),
  `watch -A --once` **ne se termine pas** (il attend une ressource non prête) alors que notre Kustomization est `ReconciliationSucceeded`. Les relevés examinés
  montrent qu'il attendait `Kustomization/sylva-system/sylva-units-status` (ou celle de `my-workload-kubeadm-capd`), une unité de Sylva qui se réconcilie
  périodiquement ; pour N2 la sortie a été coupée avant la fin de la liste (unité en cause : SUPPOSÉ). Ces relevés **ne sont pas comptés** comme
  signalement de nos scénarios. Ils montrent qu'avec `-A`, l'état « pas tout prêt » de sylvactl mélange notre cœur 5G et toutes les unités de Sylva.

## 2. Trois listes

### Où SylvaTwin voit ce qu'aucun autre outil ne voit

Scénarios où ni Flux (nomme), ni sylvactl, ni `kubectl diff -k`, ni Alertmanager ne signalent rien :

* **K2, K3** — Secret référencé supprimé (puis pod en échec) : hors Git, la Kustomization reste prête. SylvaTwin 10/10, tous les autres 0 (VÉRIFIÉ).
* **K4** — nœud qui perd le label exigé par un `nodeSelector` : aucun outil ne compare nœuds et exigences de placement. SylvaTwin 10/10, autres 0 (VÉRIFIÉ).
* **N2** — incohérence entre NF introduite **par Git** : Flux applique le commit sans erreur (c'est l'état voulu), sylvactl reste « All Done », `kubectl diff` est
  vide par construction. SylvaTwin 10/10 (VÉRIFIÉ).
* **F1** n'est **plus** dans cette liste : sylvactl signale la suspension 2/2 (VÉRIFIÉ). Il en sort grâce à sylvactl, pas à Flux (0/10).
* Avec la définition d'annexe (« activité Flux dans la fenêtre »), la liste serait F1, K4, N2 avant sylvactl, et **K4, N2** avec sylvactl.

### Où SylvaTwin fait comme les autres (il ajoute le verdict et les impacts)

* **F3** : Flux (10/10, message exact du dry-run) et sylvactl (2/2) le signalent. SylvaTwin ajoute le verdict Suspected puis Confirmed à 420 s et les 10 NF
  touchées (VÉRIFIÉ) ; il le voit ~6 s après l'échec de Flux, donc pas avant lui.
* **F1, F2** : sylvactl affiche `Suspended` dès t0 + 8 s (2/2) ; `kubectl diff` voit le scale de F2 (10/10). SylvaTwin ajoute le verdict Confirmed et l'impact
  sur 10 NF (VÉRIFIÉ).
* **K1, N1** : Flux corrige l'écart (10/10) et `kubectl diff` le voit (10/10, 9/10). SylvaTwin ajoute l'impact par dépendance (9 NF dépendant du NRF pour K1)
  et l'incohérence entre NF pour N1 (VÉRIFIÉ).
* **K5** : `kubectl diff` voit la ConfigMap modifiée (9/10) et Flux la remet (10/10). SylvaTwin est **le seul à voir la conséquence** : le pod garde l'ancienne
  configuration, la StaleConfigDrift persiste après la correction de Flux (10/10, VÉRIFIÉ). C'est un apport réel, mais pas un « seul à voir l'écart ».

### Où SylvaTwin est MOINS bon

* **H1, H2 (hors modèle)** : SylvaTwin 0/10 par construction (SpecDrift ne compare que `replicas`) alors que Flux les corrige (10/10), `kubectl diff` voit H1
  (9/10) et Flux nomme `scp` (5/10, 7/10) (VÉRIFIÉ). SylvaTwin ne remplace pas un diff complet des manifestes.
* **N1 rép 1** : faux négatif. Flux a remis la ConfigMap 3,4 s après l'injection, avant la fin de la lecture du premier cycle (VÉRIFIÉ). Une dérive plus courte
  qu'environ 3,5 s peut être manquée (`docs/LIMITES.md`, L2). Flux, lui, l'a corrigée.
* **Coût d'un cycle** : environ 2 s par cycle (médiane 2,05 s, p95 3,6 s ; lecture des deux clusters ~1,3 s), rejoué à chaque événement (~2 300 cycles par jour de
  campagne) (VÉRIFIÉ). sylvactl et Flux ne coûtent rien de plus que leur réconciliation.
* **Mémoire d'Oxigraph** : croissance continue (≈ +47 Mo par manche de 1 h 20 ; 25 → 100 Mo pendant la seconde manche de la campagne sylvactl) ; aucune des
  trois variantes de chargement ne la stabilise ; contournement par redémarrage entre manches (VÉRIFIÉ). Cause : dictionnaire non libéré (SUPPOSÉ).
* **Couche L1 (Machine ↔ Node)** : seulement en **rejeu** sur faits construits (C1), jamais injectée en réel (VÉRIFIÉ) ; le second nœud de test n'est pas fait.
* **Confirmed fugace d'un cycle pendant un rollout** (K4 restauration, 2/10, hors fenêtre) : lecture non atomique (`docs/LIMITES.md`, L1 ; cause SUPPOSÉ).
* **Délai sur les changements Git** : depuis le push, médiane 77 s (p95 129 s) ; SylvaTwin attend que Flux applique (ou échoue), puis ~6 s (VÉRIFIÉ). Il ne
  voit pas un commit avant Flux.

## 3. Ce que regarde chaque outil (une phrase chacun)

* **SylvaTwin** : l'état réel des deux clusters (management et workload) et l'état voulu (Git rendu par Flux), mis en faits, jugés par des règles SPARQL sur
  des relations entre objets (référence, placement, configuration, NF) ; il voit donc les écarts **relationnels** et leurs **conséquences**, mais seulement ceux
  qu'une règle décrit, et seulement dans l'état qu'il lit à chaque cycle (VÉRIFIÉ pour le fonctionnement ; couverture = les règles existantes).
* **Flux CD** : compare les objets qu'il gère à Git et les ré-applique ; il ne voit ni ce qui n'est pas dans Git (Secret K2, nœud K4), ni les conséquences
  (pod K5), ni une incohérence venant de Git lui-même (N2), et ne signale rien quand il est suspendu (F1) (VÉRIFIÉ sur les mesures).
* **sylvactl** : affiche l'état `Ready` des **ressources Flux** du management (325 objets) ; il voit donc tout ce qui rend une Kustomization non prête (suspension,
  échec d'application : F1, F2, F3) et rien de ce qui se passe à l'intérieur d'une Kustomization prête (K, N, H). Il n'est **pas conçu** pour la dérive de
  configuration : ses « 0/2 » ne sont pas des échecs (VÉRIFIÉ sur les mesures ; intention de l'outil : SUPPOSÉ, d'après son `--help`).
* **`kubectl diff -k`** : compare les manifestes de Git au cluster, champ par champ ; il voit tout écart sur un champ présent dans Git (K1, K5, N1, H1, F2), rien
  hors Git (K2, K3, H2), rien sur les nœuds (K4), et il est sans objet pour un écart venu de Git (F3, N2) (VÉRIFIÉ).
* **Alertmanager** (règles Prometheus déjà installées) : alerte sur des symptômes d'exécution (redémarrages, réplicas indisponibles) ; il ne voit presque aucune
  dérive de configuration (5/100) et alerte sur des opérations normales (5/40 bénins) (VÉRIFIÉ).

## 4. Conclusion

1. On peut affirmer : sur ce banc, SylvaTwin détecte 99/100 dérives de son modèle avec un verdict conforme (et 32/32 dans la campagne sylvactl), et 0 fausse alerte dans la fenêtre de mesure sur 192 répétitions (160 + 32) (VÉRIFIÉ).
2. On peut affirmer : 4 scénarios sur 10 (K2, K3, K4, N2) ne sont signalés par aucun des quatre autres outils, et pour K5 seul SylvaTwin voit la conséquence (VÉRIFIÉ).
3. On ne peut pas affirmer que SylvaTwin « fait mieux que Flux ou sylvactl » en général : ils ne regardent pas la même chose, et SylvaTwin est aveugle hors de ses règles (H1, H2).
4. On ne peut pas affirmer une détection sans trou : une dérive de moins de ~3,5 s peut être manquée (1/160), et la couche L1 n'est validée qu'en rejeu.
5. sylvactl est mesuré sur 2 répétitions seulement : ses résultats sont exacts mais à présenter en k/2, sans généralisation.
