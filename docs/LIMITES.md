# Limites connues de SylvaTwin (constatées sur la campagne à 10 répétitions, 15–16/11/2026)

Chaque limite est datée, avec sa chronologie. **Aucune correction n'est appliquée** dans ce document : les propositions attendent un accord.

## L1. Confirmed fugace pendant un rollout (K4, restauration, rép 3 et 8) — observation non atomique

**Constat (VÉRIFIÉ, `campagne-v2.3-10reps/K4/rep3` et `rep8`).** Pendant la restauration de K4 (`git revert` du commit qui ajoute
`nodeSelector sylvatwin.orange.com/zone=eval` à `bsf`), **un seul cycle** voit une `PlacementDrift` (et son `FunctionImpactDrift` sur bsf) sur un
**nouveau** pod `bsf-78db7b6587-*`, issu du ReplicaSet **sans** `nodeSelector`. Le cycle suivant (5 s plus tard) ne la voit plus.
PlacementDrift est une `UnattendedDrift` (horizon infini) : elle est donc **Confirmed tout de suite**. Hors fenêtre de mesure : elle ne compte
ni comme détection ni comme fausse alerte (transitoire de restauration), mais un opérateur l'aurait vue passer.

Chronologie, rép 3 (15/11, UTC) :

| instant | ce qui se passe |
|---|---|
| 14:00:35.568 | t0 : retrait du label de test ; PlacementDrift attendue sur `bsf-84f658bb7f-vfx44` (pod du ReplicaSet avec `nodeSelector`), Confirmed |
| 14:02:14 → 14:04:13 | la dérive attendue reste présente, seule |
| ~14:04 | `restore.sh` : retrait du label (déjà absent), `git revert` du commit de préparation, push |
| 14:04:20 | **un cycle** voit `bsf-84f658bb7f-vfx44` **et** `bsf-78db7b6587-wdjks` (nouveau pod, ReplicaSet sans `nodeSelector`) en PlacementDrift, Confirmed |
| 14:04:25 | plus aucune dérive nouvelle |

Rép 8 : même schéma (t0 20:00:00.752 ; cycle fautif 20:03:38 avec `bsf-78db7b6587-wmfzf` ; propre à 20:03:43).

**Cause (SUPPOSÉ).** La lecture d'un cycle (`observe`, ~1,3 s) n'est pas atomique : le Deployment `bsf` est lu **avant** que le revert soit appliqué
(il exige encore le label), le nouveau pod est lu **après** sa création. La règle `derives/03` relie le pod au Deployment par `:ownedBy+` et prend
l'exigence de label **sur le Deployment** : elle juge un pod neuf avec l'exigence de l'ancien gabarit.

**Proposition (non appliquée) — sans changer le sens de la règle 03 (« un pod tourne sur un nœud qui n'a pas le label que son contrôleur exige ») :**
prendre l'exigence **sur le pod lui-même**. `spec.nodeSelector` d'un pod est immuable et c'est exactement l'exigence avec laquelle il a été placé
(« IgnoredDuringExecution » porte sur le pod). Concrètement : le mapper (fonction pure) écrit aussi `:requiresNodeLabel` sur chaque Pod à partir de son
`spec.nodeSelector` ; la règle 03 lit `?pod :requiresNodeLabel ?req` au lieu de `?deployment :requiresNodeLabel ?req` et garde le Deployment
dans `:affects`. Le pod et son exigence viennent alors du **même objet**, lu en une fois : l'incohérence entre deux lectures disparaît.
Effet attendu : K4 inchangé (le pod fautif porte le `nodeSelector`) ; le pod neuf sans `nodeSelector` n'est plus en dérive. À valider par
`TestA8Equivalence` sur les faits archivés et par une répétition de K4. Alternative écartée : exiger deux cycles consécutifs avant d'ouvrir une
UnattendedDrift (retarde toutes les détections d'un cycle).

## L2. Dérive plus courte qu'environ 3,5 s : peut être manquée (N1, rép 1)

**Constat (VÉRIFIÉ).** N1 rép 1 (15/11) : injection à 10:58:45.569 ; événement reçu par SylvaTwin à 45.776 ; début du cycle à 47.777 (attente fixe
de **2,0 s** après l'événement) ; lecture `observe` de 47.78 à ~49.14 (1 359 ms) ; **Flux remet la ConfigMap à 48.949** (t0 + 3,4 s, réconciliation
périodique tombée là) ; fin du cycle à 49.928 sans dérive nouvelle. Cycles suivants : rien. La dérive n'a existé que ~3,4 s.

**Limite.** Un cycle ne voit l'état qu'au moment où il le lit : une dérive qui disparaît avant la fin de la lecture du premier cycle qui suit
l'événement (**attente de 2 s + lecture d'environ 1,3 à 1,5 s, soit environ 3,5 s**) peut être manquée. SylvaTwin juge un état, il ne rejoue pas
les événements intermédiaires. Fréquence observée : 1 répétition sur 160 (1 sur 10 pour N1). Ce n'est pas un défaut des règles.

Pistes (non appliquées) : réduire l'attente de 2 s (plus de cycles) ; ou garder la trace des événements reçus (nom de l'objet et champs changés, déjà
journalisés dans `triggerObjects`) pour signaler « objet modifié puis remis » sans le juger — ce serait une information, pas une dérive.
