# Vérifications sur la campagne à 10 répétitions — partie A (20/11/2026)

Lecture seule de `campagne-v2.3-10reps` (160 répétitions). Les résultats publiés (`docs/resultats-v2.3-10reps/`) ne sont pas modifiés ; ce
document fixe les **définitions retenues** (décision du 20/11) et les chiffres recalculés.

## « Flux signale » — définition principale : « Flux nomme l'objet ou l'écart »

Une ligne dans la fenêtre (t0 → avant `restore.sh`) compte si elle **cite l'objet injecté et l'écart en cours** (événement de la Kustomization,
`Ready=False`, ou erreur du journal du kustomize-controller). En annexe : « activité Flux dans la fenêtre » = critère d'origine de `analyse.py`
(tout événement `Warning`, ou `Normal` contenant configured/created/deleted pour un scénario non Git, `Ready=False`, ou ligne d'erreur).

| | F1 | F2 | F3 | K1 | K2 | K3 | K4 | K5 | N1 | N2 | H1 | H2 | B1 | B2 | B3 | B4 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **Flux nomme l'objet ou l'écart** | 0/10 | 0/10 | 10/10 | 2/10 | 0/10 | 0/10 | 0/10 | 4/10 | 6/10 | 0/10 | 5/10 | 7/10 | 0/10 | 4/10 | 0/10 | 0/10 |
| annexe : activité Flux dans la fenêtre | 0/10 | 4/10 | 10/10 | 2/10 | 4/10 | 3/10 | 0/10 | 4/10 | 6/10 | 0/10 | 6/10 | 7/10 | 0/10 | 4/10 | 1/10 | 0/10 |

Totaux : dans le modèle 22/100 (annexe 33/100) ; hors modèle 12/20 (13/20) ; bénins 4/40 (5/40).
Les écarts viennent de réconciliations ordinaires d'autres objets (K2, K3) et d'un `Warning` de F3 (« Bogus ») relu pendant F2, B3, H1 (aucune
erreur dans le journal à ces instants : VÉRIFIÉ ; relecture d'un ancien événement : SUPPOSÉ). Collecteur corrigé le 20/11 (voir plus bas).

**Scénarios qu'aucun autre outil ne signale** (Flux, `kubectl diff -k`, Alertmanager) :

| définition de « Flux signale » | scénarios |
|---|---|
| **Flux nomme l'objet ou l'écart** (principale) | **F1, K2, K3, K4, N2** |
| activité Flux dans la fenêtre (annexe) | F1, K4, N2 |

## « Flux corrige » — deux colonnes séparées

| | F2 | F3 | K1 | K2 | K3 | K5 | N1 | N2 | H1 | H2 | B2 | B3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Flux applique un commit (révision = commit d'injection) | – | 0/10 (rejeté) | – | – | – | – | – | 10/10 | – | – | – | 10/10 |
| Flux corrige un écart avec Git (même révision, objet ré-appliqué) | 0/10 | n/a | 10/10 | 0/10 | 0/10 | 10/10 | 10/10 | n/a | 10/10 | 10/10 (a) | 7/10 (b) | n/a |

Dans le modèle, « corrige un écart avec Git » : **30/60** (F2, K1, K2, K3, K5, N1) ; l'ancienne colonne donnait 40/80 parce qu'elle comptait
l'application du commit de N2. (a) H2 : `revisionHistoryLimit` absent de Git, retour de la valeur SUPPOSÉ. (b) B2 : Flux retire l'annotation de
redémarrage (SUPPOSÉ, constaté sur K5 le 14/11).

**K5 : la StaleConfigDrift persiste après la correction de Flux : 10/10** (VÉRIFIÉ : Flux remet `amf-config` entre +3 et +91 s ; la dérive
`StaleConfigDrift` sur le pod amf est présente, Confirmed, dans tous les cycles suivants jusqu'à la fin de la fenêtre).

## Autres points

* Dérives Suspected passagères, délais séparés, chronologie de N1 rép 1 : rapport de la partie A ; limites dans `docs/LIMITES.md`.
* **Collecteur d'événements Flux corrigé** (`eval/v2/campagne.py`, `fluxevents`) : lit la date propre de l'événement et ignore dans `events.log`
  ceux antérieurs au démarrage du collecteur (donc à t0) ; tout est gardé dans `events-brut.jsonl`. **Non appliqué aux résultats existants.**
  Ce qu'il aurait retiré : les anciens fichiers ne portent pas la date propre des événements, le compte exact est donc impossible ; les lignes
  visées (le `Warning` « Bogus » de F3 hors F3) sont **6**, dont **5** dans une fenêtre de mesure (F2 rép 3 et 5, B3 rép 2, H1 rép 4 et 10). SUPPOSÉ :
  d'autres anciens événements relus (non identifiables) ont pu exister.
