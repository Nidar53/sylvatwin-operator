# Diagnostic — verdict de GitOpsSyncDrift (F1, F2, F3) — 14/11/2026

**Diagnostic seul : aucune règle, ontologie ni code de l'opérateur n'a été modifié. En attente d'approbation.**
VÉRIFIÉ = lu dans les données/règles ; SUPPOSÉ = déduit.

## Constat (VÉRIFIÉ)

| rép | F1 | F2 | F3 |
|---|---|---|---|
| 1 | Confirmed (12:07) | Confirmed (12:19) | Suspected → Confirmed à **421 s** (12:38:05 → 12:45:06) |
| 2 | Suspected (13:35) | Suspected (14:24) | Suspected → Confirmed à **421 s** |
| 3 | Suspected (15:19) | Suspected (15:37) | Suspected → Confirmed à **420 s** |

Ordre réel des occurrences de la dérive (même identifiant) d'après `history.nt` : F1r1 12:07, F2r1 12:19, **F3r1 12:38**, F3r2 13:21, F1r2 13:35, F2r2 14:24, F3r3 15:08, F1r3 15:19, F2r3 15:37.
**Toutes les occurrences avant F3r1 sont Confirmed, toutes celles après sont Suspected.**

## Quelle règle fixe l'horizon à chaque fois

* F1r1, F2r1 : Kustomization **suspendue** → `lifecycle/03-horizon.ru` ne calcule rien (filtre `suspended = false`), `05-horizon-infini.ru` n'a rien à retirer. Pas de `:convergenceHorizon` → `verdict/01` : `!BOUND(?h)` → **Confirmed**. Conforme à l'oracle et à l'intention écrite dans 05 (« sur une Kustomization suspendue, 03 n'écrit pas d'horizon → infini »).
* F3 (r1, r2, r3) : Kustomization **active** (le dry-run échoue, elle n'est pas suspendue) → `03` écrit `interval 120 + timeout 300 = 420 s` → Suspected puis Confirmed quand l'âge atteint 420 s. Mesuré : 421, 421, 420 s (écart ≤ 1 s = un cycle). **F3 est conforme à l'oracle** ; l'âge part du premier cycle qui voit la dérive (première tentative en échec de Flux : 4 à 128 s après le push), pas du push.
* F1/F2 r2, r3 : la Kustomization est de nouveau suspendue, `03` n'écrit rien… mais **le `:convergenceHorizon 420` écrit par F3r1 est resté dans le graphe history** (`history.nt` en contient un seul, pour cette dérive, et il est encore là). Aucune règle ne le retire : `01` ne supprime que `lastDetectedAt` et `resolvedAt` ; `02` n'ajoute que `resolvedAt` ; `05` ne vise que les dérives non surveillées et les ressources hors Git (les Kustomization sont exclues du cas (a)). La nouvelle occurrence reçoit `firstDetectedAt = maintenant` et l'horizon périmé 420 → **Suspected** pendant 420 s.

## Ce qui diffère entre la répétition 1 et les suivantes (VÉRIFIÉ)

* **L'historique**, seulement. Ni l'ordre des événements, ni l'instant du fait `:suspended` : la dérive est vue 4 s après t0 aux 3 répétitions de F1 (la mise à jour de la Kustomization déclenche le cycle, le fait et la dérive naissent dans le même cycle).
* Ce n'est pas la répétition qui compte mais le fait que **F3r1 est passé avant** : si F3 avait été exécuté après F1 et F2 de la manche 1, F1r2 et F2r2 auraient été Suspected quand même (SUPPOSÉ) ; l'ordre aléatoire de la campagne explique que r1 soit Confirmed.

## Deuxième défaut, plus large (VÉRIFIÉ)

`01-ouvrir-ou-prolonger.ru` : « une dérive résolue puis revenue repart de zéro » est écrit en commentaire, mais la règle **ajoute** un nouveau `:firstDetectedAt` sans supprimer l'ancien. `history.nt` contient **9** `firstDetectedAt` pour la dérive GitOpsSync (une par occurrence). `verdict/01` produit un verdict **par** `firstDetectedAt` : l'ancienne occurrence (âge de plusieurs heures) donne **Confirmed**, la nouvelle **Suspected**, avec le **même identifiant de verdict**.

* Preuve par le journal du contrôleur : à 12:38:05 et 13:35:04, `driftsByVerdict[GitOpsSyncDrift] = {Confirmed: 1, Suspected: 1}` pour une seule dérive ; 27 Confirmed + 11 Suspected pour 21+1+5 dérives.
* Le banc v2 garde un seul verdict par dérive (le dernier de la liste) : le Confirmed en double était **masqué** et on lisait Suspected. Les données v2 ne permettent pas de dire lequel s'affichait « officiellement ».
* Étendue (VÉRIFIÉ, journal, cycles avec plus de verdicts que de dérives) : tous les scénarios dont la dérive revient : F1, F2, F3 ; K1 (SpecDrift + 10 impacts) ; K3 rép 2 et 3 (BrokenReference + ConvergenceDrift) ; B1, B3 (ConvergenceDrift transitoire).
* Conséquence pour les résultats v2 : les « verdict conforme » de K1, K3 (rép 2–3), B1 et B3 reposent sur un verdict que le banc a choisi parmi deux. Les détections, délais, impacts et fausses alertes (par classe) ne sont pas touchés.
* Le banc v2.1 enregistre désormais tous les verdicts d'un même identifiant (`ver_multi`).

## Défaut des règles ou attente fausse de l'oracle ?

**Défaut des règles (cycle de vie), pas de l'oracle.** L'oracle (« Confirmed tout de suite » pour une suspension) suit l'intention documentée de `05-horizon-infini.ru` et du commentaire de `01`. Deux défauts distincts : (B) un horizon périmé survit à la résolution ; (A) les `firstDetectedAt` d'occurrences passées survivent.

## Proposition (NON appliquée — à approuver)

Dans `rules/lifecycle/01-ouvrir-ou-prolonger.ru` : quand la dérive est **réouverte** (`?res` lié), supprimer aussi, dans history, l'ancien `:firstDetectedAt` et l'ancien `:convergenceHorizon`. Ainsi chaque occurrence repart de zéro comme annoncé, `03` recalcule l'horizon si la Kustomization est active, et une suspension n'a plus d'horizon (Confirmed immédiat). Une seule règle, additive. À faire avec : test unitaire « dérive résolue puis revenue sous Kustomization suspendue → Confirmed au 1er cycle, un seul verdict », et re-mesure de F1, F2, K1, K3.
Alternative plus petite : seulement `03` (supprimer l'horizon quand il n'est pas calculable). Elle règle F1/F2 mais pas les verdicts en double ; je ne la recommande pas.
