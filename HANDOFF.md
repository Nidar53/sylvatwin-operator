# HANDOFF — branche `test2` (mis à jour le 13/11/2026)

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
