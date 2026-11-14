# Correction du cycle de vie (réouverture, horizon recalculé) — 14/11/2026

Dépôt poc, branche test2 : `54b7e65` (règles + tests) et `faed569` (correctif de test : les faits de réouverture étaient pris pour des variantes de dérive). Diagnostic : `DIAGNOSTIC-SUSPENSION-2026-11-14.md`. **Non déployé** au moment de l'écriture (campagne2 en cours) ; déploiement par `eval/v2/enchainement-v22.sh`.

## Règles (2 fichiers ajoutés, aucun fichier existant modifié, TBox inchangée)
* `rules/lifecycle/00-reouverture-raz.ru` : pour une dérive du cycle qui a un `:resolvedAt`, supprime TOUS ses `:firstDetectedAt` (01 écrit ensuite le nouveau).
* `rules/lifecycle/02b-horizon-raz.ru` : pour toute dérive présente dans `<drifts>`, supprime son `:convergenceHorizon` avant 03/04. Les dérives absentes du cycle (cluster injoignable) gardent leur horizon.

## Tests avant déploiement (AVANT = règles 37601b5 ; APRÈS = 54b7e65/faed569)
| test | avant | après |
|---|---|---|
| rdflib `verif_v1.py` section 9 : cas 1 (rouverte sous suspension), cas 2 (suspension sans résolution), cas 3 (rouverte sous Ks active, âge depuis la réouverture), invariants | 7 échecs sur 8 | 8/8 OK, reste de verif_v1 inchangé |
| Oxigraph `TestConformiteReouverture` (17 étapes, identiques à rdflib) | séquence identique à rdflib MAIS 8 étapes avec 2–3 `firstDetectedAt` et verdicts faux (ex. cas 1 t=200 : Suspected au lieu de Confirmed) | OK, toujours 1 `firstDetectedAt` |
| `TestConformiteRdflibOxigraph`, `TestConformiteVerdict`, tests judge existants | OK | OK (suite `internal/judge` complète) |
| `TestEtatSainInchange` (5 StaleConfigDrift + 11 FunctionImpactDrift, mêmes firstDetectedAt, 16 verdicts Confirmed) | — | exécuté par l'enchaînement sur un instantané réel de l'état sain (impossible pendant campagne2 : l'état n'est pas sain) |

## Rejeu des répétitions de la campagne v2 (aux dates réelles ; `tests/rejeu_campagne_v2.py`)
LIMITE : les faits complets n'avaient PAS été archivés (seulement ids et verdict par cycle) ; faits reconstruits à partir des variantes (F1, F2 : S5-suspend, sans le scale de udr ; F3 : S5-retard ; K1 : S0A-nrf ; K3 : S2 + S0B-udm).
| rép | avant (rejoué) | après (rejoué) | enregistré par le banc v2 (dernier verdict) |
|---|---|---|---|
| F1 / F2 rép 1 | Confirmed | Confirmed | Confirmed |
| F1 / F2 rép 2, 3 | Confirmed **+** Suspected (2 verdicts) | **Confirmed**, 0 double | Suspected |
| F3 rép 1–3 | Suspected (+ Confirmed en double dès la rép 1) → Confirmed | Suspected → Confirmed, 0 double | Suspected → Confirmed |
| K1 rép 2, 3 | Suspected + Confirmed (doubles) | Suspected, 0 double | Suspected |
| K3 rép 2, 3 | doubles | 0 double | — |
Le rejeu « avant » reproduit le défaut observé ; le rejeu « après » donne les verdicts de l'oracle.

## Invariant dans le banc (pas en Go)
`campagne.py` enregistre à chaque cycle `fd_multi` (dérive avec plus d'un `firstDetectedAt`, lu dans `out/history.nt`) et `ver_multi` (dérive avec plus d'un verdict) ; `analyse.py` les compte (« Invariants des règles ») ; `verif_sain.py --strict` les exige nuls.
