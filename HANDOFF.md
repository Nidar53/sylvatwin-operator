# HANDOFF — arrêt pour saturation de contexte (10/11/2026)

## Où j'en suis
- Partie 3 (corrections) : **terminée et commitée sur `test`** (commits `1354b30` poc, `71d1683` operator). Vérifications vertes : `verif_v1.py`, ROBOT, HermiT, `TestConformiteRdflibOxigraph`, `TestIsomorphismeExemple`.
- Partie 1 (contrôleur temps réel) : code écrit (`internal/watcher`, `cmd/sylvatwin-controller`), **compile**, mais **jamais testé en conditions réelles** (ni envtest, ni sur le vrai cluster). Commité sur la branche **`wip-temps-reel`** (commit `a75b584`), pas sur `test`.
- Je m'apprêtais à lancer un essai manuel du contrôleur dans tmux quand le contexte s'est rempli — **aucune commande d'essai n'a été exécutée**, rien n'a tourné.

## Branche wip-temps-reel (poussée sur GitHub)
Contient : `cmd/sylvatwin-controller/main.go` (point d'entrée, flags, journal JSONL) + modif de `internal/watcher/watcher.go` (rattachement du Watch témoin `WitnessNode`/`FreshnessHorizon` à la première source workload engagée).
À faire avant merge dans `test` : test réel (voir "prochaine étape"), puis `git checkout test && git merge wip-temps-reel`.

## Ce qui tourne sur le serveur
- Rien lancé pendant cette session (la dernière commande a été refusée avant exécution).
- Vérifier quand même avant de continuer : `tmux ls` (sessions possibles d'une session précédente : `sylvatwin-oxigraph`). Si présente : contient la base Oxigraph de test, `tmux kill-session -t sylvatwin-oxigraph` pour l'arrêter, `rm -rf /opt/nidar/sylvatwin-run/oxigraph-data` pour la vider.
- `bin/sylvatwin-controller` et `bin/sylvatwin-once` compilés dans `/opt/nidar/src/sylvatwin-operator/bin/` (ignorés par Git, à garder ou supprimer librement).

## Pièges déjà rencontrés et corrigés (dans `test`, pour mémoire)
- rdflib 7.6 : `GROUP BY` sans résultat + `OPTIONAL` externe → fausse ligne. Fix : `HAVING (COUNT(...) > 0)` (voir `rules/lifecycle/README.md`).
- Un `BIND` du bloc extérieur n'est pas visible dans un sous-`SELECT` (scope séparé) : il faut re-`BIND` `?now` à l'intérieur.
- `multicluster-runtime/providers/cluster-api` est un **sous-module Go séparé** (`sigs.k8s.io/multicluster-runtime/providers/cluster-api`), avec son propre `go.mod`, pas dans l'arborescence `providers/` du dépôt racine cloné — chercher via `go list -m -versions`.
- Le test de garde des Secrets (`secrets_guard_test.go`) est sensible à la présence littérale de `Resource: "secrets"` : toute nouvelle `Spec` doit porter `MetadataOnly: true` **sur la même ligne**.

## Prochaine étape (reprendre ici)
1. Relire `internal/watcher/watcher.go` et `cmd/sylvatwin-controller/main.go` sur `wip-temps-reel` (pas encore relus après écriture).
2. Lancer un essai réel COURT (30 s) dans tmux : Oxigraph sur 127.0.0.1:7878, puis `sylvatwin-controller --mgmt-kubeconfig management-cluster-kubeconfig --rules .../sylvatwin-poc/rules --sparql http://127.0.0.1:7878 --out ... --journal ... --witness-node my-workload-kubeadm-capd-control-plane-6w58l --freshness-horizon 30s --debounce 2s`, en lecture seule (aucune injection). Vérifier : connexion au workload via CAPI + impersonation, au moins un cycle déclenché, journal JSONL cohérent.
3. Si ça marche : écrire les tests envtest de la partie 1.4, puis la barrière 2h (partie 1.5), avant de merger dans `test`.
4. Si ça ne marche pas : déboguer avant toute chose (ne pas avancer sur les parties 2/4/5/6/7/8 tant que le temps réel n'est pas validé — c'est l'exigence non négociable de la mission).
5. Resterait ensuite à faire en entier : parties 4 (audit 24h, déjà des pistes trouvées à la main dans la conversation : écart de version Machine/kubelet, DriftDetected réel sur rancher-backup-restore-operator, D4 réel confirmé par exec sur nrf/smf), 5 (génération SHACL), 6 (nouvelle campagne), 7 (ablation), 8 (rapport final).
