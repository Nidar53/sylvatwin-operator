# Évaluation v2 — mode d'emploi (banc `eval/v2/`)

Oracle : `eval/v2/criteres-v2.md` + `scenarios/<id>/oracle.json` (écrits avant tout essai, **ne pas les modifier après un essai**).
Cadre : namespace `sylvatwin-5gc`, Kustomization `sylvatwin-flux/sylvatwin-5gc`, branche `test` de sylvatwin-poc dans `usecase/5gc` (F3, N2, B3 seulement, via le clone `/opt/nidar/sylvatwin-run/eval-v2/gitwork`).

## Suivre la campagne
```sh
tmux attach -t campagne                      # (Ctrl-b d pour détacher)
grep -v '^  ' /opt/nidar/sylvatwin-run/eval-v2/campagne-v2.log | tail      # une ligne par étape
ls /opt/nidar/sylvatwin-run/eval-v2/campagne-v2/*/*/marks.tsv              # répétitions faites
python3 /opt/nidar/src/sylvatwin-operator/eval/v2/analyse.py /opt/nidar/sylvatwin-run/eval-v2/campagne-v2   # résultats partiels : resultats-v2.md / .json dans le dossier
```
L'ordre est manche par manche (3 manches, graine dans `plan.json`) : une manche terminée est complète même si on arrête avant la fin.

## Arrêter proprement
`touch /opt/nidar/sylvatwin-run/eval-v2/STOP` : la campagne s'arrête **après** la répétition en cours (restaurée, état sain vérifié).
Ou `tmux send-keys -t campagne C-c` : la répétition en cours est restaurée (scripts `restore.sh`), puis arrêt. Ne pas faire `kill -9`.

## Tout annuler (à la main, seulement si la campagne est arrêtée)
Si `/opt/nidar/sylvatwin-run/eval-v2/ETAT_BANC.txt` existe, une restauration a échoué : la campagne s'est arrêtée sans rien tenter d'autre.
Dans l'ordre, pour le scénario indiqué : `RUNDIR=<dossier de la répétition> REP=<n> eval/v2/scenarios/<id>/restore.sh` (idempotent), puis
```sh
eval/v2/prepare.sh --sans-redemarrage-oxigraph      # vérifie : Flux appliqué, label retiré, udm-keys présent, 5+11 dérives
git -C /opt/nidar/sylvatwin-run/eval-v2/gitwork fetch -q origin test && git -C /opt/nidar/sylvatwin-run/eval-v2/gitwork diff --stat $(cat /opt/nidar/sylvatwin-run/eval-v2/START_SHA) origin/test -- usecase/5gc   # doit être vide
```
(un commit d'injection Git non annulé se défait par `git revert`, jamais `reset` ni `push --force`).

## Relancer un scénario seul / refaire avec 10 répétitions
```sh
cd /opt/nidar/src/sylvatwin-operator/eval/v2
./campagne.sh --scenario K1 --reps 3 --outdir /opt/nidar/sylvatwin-run/eval-v2/k1-seul        # un scénario
./campagne.sh --scenario K1 --scenario K2 --reps 3 --seed 5 --outdir …                         # plusieurs
./campagne.sh --dry-run --reps 3                                                                # contrôle sans rien modifier
./prepare.sh && tmux new -d -s campagne './campagne.sh --reps 10 --outdir /opt/nidar/sylvatwin-run/eval-v2/campagne-v2-10reps > …/campagne-v2-10reps.log 2>&1'
```
Durée : environ 1 h 40 par manche (K3 ≈ 13 min et F3 ≈ 12 min à cause des horizons 600 s et 420 s) : `--reps 10` ≈ 17 h.

## Où sont les mesures
`<dossier>/<scénario>/rep<n>/` : `marks.tsv` (instants), `sylvatwin/cycles.jsonl` (un cycle : dérives, verdicts, objet déclencheur), `flux/events.log` + `poll.jsonl` (toutes les 10 s),
`alerts/poll.jsonl` (Alertmanager du workload), `diffs/` (`kubectl diff -k` à t0+10 s et avant la restauration), `inject.log`, `restore.log`. `rss.txt` : RSS d'Oxigraph début/fin.
Aucun jeton, kubeconfig ni contenu de Secret n'y est écrit (le Secret udm-keys est recréé avec les clés d'exemple publiques de l'image Open5GS, sans jamais les afficher).

## Limites connues
* `flux diff kustomization` n'est pas comparable ici (il évalue contre le cluster de management, la Kustomization vise un cluster distant) : « non comparé ».
* Les événements Flux sont dédupliqués par Kubernetes : une correction de dérive silencieuse peut ne laisser aucun événement (observé sur K1 : Flux corrige sans événement).
* C1 : rejeu seulement (`eval/v2/c1-rejeu/RESULTAT.md`). K4 : retiré (voir `criteres-v2.md`). Analyseur statique : non installé.
