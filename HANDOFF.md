# HANDOFF — branche `test2` (mis à jour le 14/11/2026)

Archive de la session précédente (très détaillée, branche `test`) : `docs/HANDOFF-2026-11-10.md`.
À lire avec `README.md` (architecture, vocabulaire TBox / ABox, ordonnanceur, témoins).

## État

* Contrôleur continu : `cmd/sylvatwin-controller`. Un cycle à la fois, cause journalisée
  (`evenement` | `echeance` | `reprise`), objets déclencheurs journalisés (`triggerObjects`,
  noms de champs modifiés, jamais de valeur).
* Fraîcheur : un témoin (Watch des Lease de `kube-node-lease`) par workload découvert ; chaque
  Lease reçue **repousse** la minuterie de fraîcheur de ce cluster sans cycle ni jugement. Le Watch d'un
  témoin est arrêté quand son cluster disparaît.
* Règles : `sylvatwin-poc/rules/{derives,lifecycle,verdict,deadline}`. La requête d'échéance rend
  des **délais en secondes** (horizon de convergence, fenêtre de fraîcheur) ; Go n'ajoute que `now`.
* TBox 1.4.0 chargée dans `…/graph/tbox` à chaque démarrage d'Oxigraph (`--tbox`, contrôle de version,
  refus si < 1.4.0). `lifecycle/05` lit `rdfs:subClassOf* :UnattendedDrift`. `derives/10` garde sa liste de
  classes (lire `:RootDrift` ajouterait des impacts pour `InterNFConsistencyDrift` : vérifié sur S4).
* Oxigraph **en mémoire** ; `history.nt` est la seule persistance (âge des dérives).
* Phase C (13/11) : Go v0 supprimé (`api/`, `internal/controller`, `internal/legacyv0`, `cmd/main.go`, scaffolding
  Kubebuilder) ; prototype Python v0 et ses données dans `sylvatwin-poc/legacy/`.

## Services (systemd utilisateur, `~/.config/systemd/user/`, hors dépôt)

* `sylvatwin-oxigraph.service` : `oxigraph serve --bind 127.0.0.1:7878` (en mémoire), `Restart=always`.
* `sylvatwin-controller.service` : `Wants/After` Oxigraph, `bin/sylvatwin-controller` (`--out`, `--journal` dans un dossier daté
  de `/opt/nidar/sylvatwin-run/`), proxies HTTP dans l'environnement, `Restart=always`. Journaux horodatés dans journald.
  Les deux sont `enable`. Recompiler puis `systemctl --user restart sylvatwin-controller`.
* Vrai redémarrage d'Oxigraph vérifié (13/11) : TBox (1247 triplets) et `history` rechargés au cycle suivant, âge des 16 dérives conservé.

## Résultats de l'état sain (13–14/11/2026, dossier `test2-sain-20261113/`)

* 2 h (145 cycles, tous `evenement`) : décompte constant 5 StaleConfigDrift + 11 FunctionImpactDrift, tous Confirmed ; 0 erreur partielle ;
  0 cycle en parallèle ; 0 `echeance` (les Lease repoussent la minuterie). Cycle : médiane 1911 ms, p95 2007 ms (`observe` 1337 ms, jugement ~250 ms).
* Cycles « inutiles » (ne changent rien) : mises à jour de `[status]` de Kustomization `sylvatwin-5gc` (~40 %), Nodes, HelmRelease `sylva-units`.
  Décision du 14/11 : **ne rien filtrer**.
* Mémoire d'Oxigraph : 30 Mo → 76 Mo (2 h) → 331 Mo (13 h) alors que les triplets sont stables (facts 17 089) ; croissance linéaire
  (~0,17 Mo/cycle) et le jugement ralentit (127 → 480 ms pour `judge_lifecycle`). Voir `docs/MEMOIRE-OXIGRAPH-2026-11-14.md`. **Non corrigé** (propositions : plafond systemd, envoi du delta des faits).
* 500 d'Oxigraph sur connexion inactive (48 en 2 h, sans effet sur les cycles) : corrigé le 14/11 (`IdleConnTimeout` 1 s) ; 0 message sur 28 min
  (≈ 11 attendus), durée de cycle inchangée par rapport à l'instant précédent.
* Lecture d'un cluster bornée : `--read-timeout` (20 s) ; test unitaire serveur muet. Pas encore observé en vrai (test 3).

## `--validity` et `--freshness-horizon`

* `--freshness-horizon` (θ, 30 s) : un cluster **avec témoin** est périmé si aucune Lease n'est reçue depuis θ.
* `--validity` (600 s) : un cluster **sans témoin** (le management) est périmé s'il n'a pas été synchronisé depuis ce délai.
  Aussi utilisé par la requête d'échéance (branche sans témoin).

## Pièges (SPARQL / Oxigraph 0.5.13)

* `dateTime + dayTimeDuration` n'est **pas** évalué (binding vide, sans erreur) : utiliser `__SECONDS__`.
* `CEIL`, pas `CEILING`.
* Un `BIND` du groupe englobant n'est visible ni dans les branches d'un `UNION` ni dans un sous-SELECT.
* Un `OPTIONAL` dans un `GRAPH { … }` est évalué isolément.
* Les règles de `derives/` sont évaluées sur l'union des graphes (`default-graph-uri`) : **aucune clause `GRAPH`** (test `TestDerivesSansClauseGRAPH`).
* Oxigraph n'accepte pas un `GET` sur un graphe absent (404) : tester avec `ASK` d'abord.

## Questions ouvertes / réserves

* `InterNFConsistencyDrift` ne produit **aucun** `FunctionImpactDrift` aujourd'hui : `derives/10` garde sa liste de classes (décision du 14/11 ;
  lire `:RootDrift` ajouterait ces impacts, constaté sur S4). À rouvrir si l'article veut ces impacts.
* Mémoire d'Oxigraph (ci-dessus) : à traiter avant toute observation longue.
* Retour d'une Lease après péremption → rejugement immédiat : testé en unitaire seulement. Délai « inconnu » avec connexion muette : ≤ ~53 s (SUPPOSÉ).
* `TestA8Equivalence` exécuté sur un cycle sur deux (76 cycles). Les corrections 1, 2 et 5 de la phase A sont dans un seul commit.
* Trois tests de l'auteur (scale ; Kustomization suspendue puis scale ; coupure puis connexion muette) : préparés dans `docs/PROCEDURE-TESTS.md`, non exécutés.

## Reste à faire (recherche)

* **Partie 4** — chasse aux erreurs réelles dans l'état sain (24 h d'observation, lecture seule, deux clusters) ; ajouter les règles manquantes.
* **Partie 6** — scénarios fondés sur des sources citables (Reddit Pi-Day 2023, Zhang et al. arXiv 2512.05062, k8s.af), puis campagne en temps réel (5 répétitions, bénins B1 à B3, `eval/criteres.md` à corriger avant).
* **Partie 7** — ablation sur les faits archivés (désiré, couche 5G, Cluster API, horizon et fraîcheur).
* **Partie 8** — rapport final.

Abandonnés (décision du 13/11/2026) : génération automatique de règles SHACL (ancienne partie 5) et synchronisation adaptative.

## Tests

Voir le README. Équivalence avant / après d'un changement de règles sur les faits archivés :
`TestA8Equivalence` (`SYLVATWIN_POC_OLD`, `SYLVATWIN_ARCHIVE`, filtres `SYLVATWIN_ARCHIVE_ONLY` / `_STEP`).
Faits archivés : `/opt/nidar/sylvatwin-run/facts-campagne-20261109.tar.gz` (extraits dans `…/test2-scratch/archive`).
