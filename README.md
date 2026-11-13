# SylvaTwin — jumeau numérique sémantique d'un cloud télécom Sylva

Contrôleur Go qui observe en **temps réel** un cluster de management Cluster API
et ses clusters workload, en construit un graphe RDF, et laisse un juge SPARQL
(Oxigraph) y détecter des **dérives** : ce que Flux, `kubectl` et les `Ready=True`
ne voient pas. Base d'un article IEEE NetSoft 2027 (contribution principale : la détection).

Dépôt compagnon : `sylvatwin-poc` (TBox OWL, règles SPARQL, tests de conformité).

## Vocabulaire

| Terme | Sens ici |
|---|---|
| **TBox** | l'ontologie `sylvatwin-core-v1.ttl` (classes, propriétés, hiérarchie des classes de dérive). Chargée dans le graphe `…/graph/tbox` d'Oxigraph à chaque démarrage, avec contrôle de version. |
| **ABox** | les faits d'un cycle : ce que le greffier a observé (cluster, Kubernetes, Flux, fonctions réseau) et le côté désiré lu dans Git. Graphe `…/graph/facts`. |
| **greffier** | le *mapper* (`internal/mapper`) : fonction pure d'instantanés vers triplets. Il écrit des faits, jamais un jugement. |
| **juge** | les règles SPARQL de `sylvatwin-poc/rules/` : `derives/` → dérives, `lifecycle/` → âge et horizon, `verdict/` → verdict daté, `deadline/` → prochaine échéance. **Aucune logique de détection en Go.** |

## Architecture

```
 Watch (multicluster-runtime, provider Cluster API) ──événement──┐
 Watch des Lease de nœud (un « témoin » par workload)            │ repousse la minuterie de fraîcheur
                                                                 ▼
                              ordonnanceur (internal/watcher/scheduler.go)
                       un seul cycle à la fois · fusion des rafales bornée · une minuterie d'échéance
                                                                 │ cause : evenement | echeance | reprise
                                    ┌────────────────────────────┴─────────────────────┐
                                    ▼ cycle complet                                    ▼ échéance
                      LIST des clusters + Git (désiré)                      rejugement sur les instantanés
                      greffier → facts.nt                                   en mémoire (0 requête API)
                                    └───────────────► juge (Oxigraph, SPARQL 1.1) ◄───────┘
                                       derives → lifecycle → verdict → prochaine échéance
```

* **Pas de boucle périodique** qui relit le cluster. Le cycle part d'un événement Watch ;
  l'échéance (fin d'un horizon de convergence, péremption d'un témoin) est calculée par
  SPARQL (`rules/deadline`) et ne rejuge que les faits déjà en mémoire. Après un échec :
  reprise à 5, 10, 20, 40, 60 s (plafond), arrêtée au premier succès.
* **Fraîcheur par cluster** : un cluster avec témoin devient « périmé » si aucune Lease n'a été
  reçue depuis `--freshness-horizon` (θ, 30 s) ; un cluster sans témoin (le management)
  au bout de `--validity` (600 s) sans synchronisation. Le verdict `Unknown` est décidé par
  `rules/verdict`, seulement si un cluster des ressources de la dérive est périmé.
* **Oxigraph en mémoire** (sans `--location`) : seul le graphe `history` (âge des dérives) est
  exporté après chaque cycle dans `<out>/history.nt` et rechargé si le serveur l'a perdu.
  L'ABox est reconstruite à chaque cycle.
* **Lecture seule** sur les clusters (impersonation de `sylvatwin-reader`).

## Binaires

| | |
|---|---|
| `cmd/sylvatwin-controller` | le contrôleur continu (voir les options : `--help`). |
| `cmd/sylvatwin-once` | un seul cycle, hors temps réel (diagnostic). |

```sh
oxigraph serve --bind 127.0.0.1:7878 &
sylvatwin-controller --mgmt-kubeconfig <kubeconfig> --rules <sylvatwin-poc>/rules \
  --sparql http://127.0.0.1:7878 --track sylvatwin-flux/sylvatwin-5gc \
  --nf-namespace sylvatwin-5gc --out <dossier> --journal <dossier>/journal.jsonl
```

Sorties par cycle dans `--out` : `facts.nt`, `drifts.nt`, `verdicts.nt`, `history.nt`, `cycle.json`
(cause, objets déclencheurs, durées par étape, requêtes API, dérives par classe et verdict,
prochaine échéance). `--journal` en garde la suite en JSONL.

## Tests

```sh
export SYLVATWIN_POC=<sylvatwin-poc> OXIGRAPH_BIN=<oxigraph>
export KUBEBUILDER_ASSETS=<dossier envtest>      # tests internal/watcher
go vet ./... && go test ./... -skip 'TestDisqueA4|TestMesureA4'
```

`TestDisqueA4` et `TestMesureA4` sont des mesures longues (disque d'Oxigraph) : à ne pas lancer par défaut.
Côté `sylvatwin-poc` : `python3 tests/verif_v1.py` (rdflib), et ROBOT pour les profils OWL 2 DL / RL de la TBox.

## Répertoires

`internal/mapper` greffier · `internal/judge` client Oxigraph et exécution des règles ·
`internal/twin` un cycle (greffier + juge) · `internal/watcher` ordonnanceur, témoins, déclencheur ·
`internal/source` lecture des clusters · `internal/desired` côté désiré (Git, Kustomization) ·
`eval/` résultats de la campagne du 09/11 · `docs/` archives.
