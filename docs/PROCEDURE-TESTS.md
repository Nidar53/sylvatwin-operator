# Procédure des trois tests (à exécuter par l'auteur — rien n'a été exécuté)

Préparée le 13/11/2026. Les commandes marquées **[INJECTION]** modifient un cluster : seul l'auteur les lance.
Tout le reste est en lecture seule.

## Avant de commencer (état sain, services en marche)

```sh
source /opt/nidar/env.sh
R=/opt/nidar/sylvatwin-run/test2-sain-20261113          # dossier de sortie daté du service
systemctl --user is-active sylvatwin-oxigraph sylvatwin-controller      # active / active
python3 /opt/nidar/src/sylvatwin-operator/scripts/suivre-journal.py $R/journal.jsonl --depuis $(date -u +%T)
```

`suivre-journal.py` affiche UNE ligne par cycle :
`heure UTC | cause | dérives (classe:nombre) | verdicts | durée | requêtes API | objet déclencheur`.
État sain attendu : `FunctionImpact:11 StaleConfig:5 | Confirmed:16`.
Autres endroits : `$R/out/cycle.json` (dernier cycle), `$R/out/verdicts.nt`, `$R/out/drifts.nt`,
`journalctl --user -u sylvatwin-controller -o short-iso -f`.

Vérifier la valeur Git du nombre de réplicas avant d'injecter :
`kubectl --kubeconfig $WL_KC -n sylvatwin-5gc get deploy nrf -o jsonpath='{.spec.replicas}'`.

---

## Test 1 — `kubectl scale`

**[INJECTION]** `kubectl --kubeconfig $WL_KC -n sylvatwin-5gc scale deploy/nrf --replicas=<N différent de Git>`

À voir dans `suivre-journal.py` :

1. Dans les 2 à 5 s (debounce 2 s + cycle ≈ 2 s) : un cycle `evenement`, objet déclencheur `… Deployment sylvatwin-5gc/nrf update [spec]`.
2. `SpecDrift:1` en `Suspected` ; `FunctionImpact` passe de 11 à 21 : 10 impacts nouveaux, tous `Suspected`
   (le NRF par `:realizedBy`, puis les 9 NF qui en dépendent par `:dependsOnNF+` — constaté le 13/11).
   Les 11 + 5 existants restent `Confirmed`.
3. Deux issues : (a) Flux remet la valeur Git (≈ 40 s le 13/11) → cycle `evenement`, retour à `FunctionImpact:11 StaleConfig:5`.
   (b) Flux ne la remet pas : à la fin de l'horizon (`interval + timeout` de la Kustomization = 2 min + 5 min d'après le cluster,
   donc ≈ 7 min), un cycle `echeance` (`api=0`) fait passer les verdicts à `Confirmed`. La date est dans `out/cycle.json` → `nextDeadline`.

**Annulation** : rien à faire si Flux a corrigé ; sinon `kubectl … scale deploy/nrf --replicas=<valeur Git>`.

---

## Test 2 — Kustomization suspendue, puis scale

**[INJECTION]** `kubectl --kubeconfig $MGMT_KC -n sylvatwin-flux patch kustomization sylvatwin-5gc --type merge -p '{"spec":{"suspend":true}}'`

1. Cycle `evenement` (`Kustomization … sylvatwin-5gc update [spec]`). Nouvelle dérive `GitOpsSync:1` (la règle 07 couvre la suspension),
   `Confirmed` tout de suite : une Kustomization suspendue n'a pas d'horizon (rules/lifecycle/03 puis 05 : horizon infini).
   Les NF gérées par cette Kustomization deviennent des `FunctionImpact` (chemin `flux`), aussi `Confirmed` : le total monte au-dessus de 11
   (SUPPOSÉ : jusqu'à 10 de plus ; lire le compte réel).

**[INJECTION]** `kubectl --kubeconfig $WL_KC -n sylvatwin-5gc scale deploy/nrf --replicas=<N différent de Git>`

2. Cycle `evenement` `Deployment nrf update [spec]` : `SpecDrift:1` **tout de suite `Confirmed`** (pas de `Suspected` :
   Flux ne corrigera pas, l'horizon est infini), impacts `Confirmed`. Aucun retour à l'état sain tant que la Kustomization reste suspendue.
   Pas d'`echeance` pour ces dérives.

**Annulation (dans cet ordre)** :
`kubectl … scale deploy/nrf --replicas=<valeur Git>` puis
`kubectl --kubeconfig $MGMT_KC -n sylvatwin-flux patch kustomization sylvatwin-5gc --type merge -p '{"spec":{"suspend":false}}'`.
Vérifier le retour à `FunctionImpact:11 StaleConfig:5 | Confirmed:16`.

---

## Test 3 — coupure puis connexion muette (SIGSTOP du proxy)

Principe : un proxy TCP local relaie vers le control-plane du workload ; le contrôleur de test lui est relié par
`--test-host-override`. Couper ou geler le proxy simule la panne **sans rien toucher sur le cluster**.
On utilise une **seconde instance** (Oxigraph et contrôleur séparés, sorties à part) pour ne pas perturber le service.

```sh
source /opt/nidar/env.sh
T=/opt/nidar/sylvatwin-run/test2-t3-$(date +%Y%m%d-%H%M); mkdir -p $T/out
S=/opt/nidar/src/sylvatwin-operator
export no_proxy=localhost,127.0.0.1,172.30.0.0/24,100.72.0.0/16,100.73.0.0/16,100.100.0.0/16,100.96.0.0/16,.sylva,.svc,.cluster.local,.cluster.local.,.orange-labs.fr,.intraorange,.francetelecom.fr,.tech.orange NO_PROXY=$no_proxy
# 1. Oxigraph jetable (mémoire) sur un AUTRE port
tmux new -d -s t3-ox "/opt/nidar/tools/oxigraph serve --bind 127.0.0.1:17879"
# 2. le proxy (affiche son PID)
tmux new -d -s t3-proxy "python3 $S/scripts/proxy-tcp.py 127.0.0.1:19443 172.30.0.2:6443 2>&1 | tee $T/proxy.log"
sleep 2; PID=$(pgrep -f 'proxy-tcp.py 127.0.0.1:19443'); echo "pid proxy = $PID"
# 3. le contrôleur de test (le management est lu directement ; seul le workload passe par le proxy)
tmux new -d -s t3-ctl "cd $S && ./bin/sylvatwin-controller --mgmt-kubeconfig $MGMT_KC --rules /opt/nidar/src/sylvatwin-poc/rules \
  --sparql http://127.0.0.1:17879 --track sylvatwin-flux/sylvatwin-5gc --nf-namespace sylvatwin-5gc --out $T/out \
  --cache /opt/nidar/cache-desired --freshness-horizon 30s --journal $T/journal.jsonl --metrics-bind-address :0 \
  --test-host-override 127.0.0.1:19443 2>&1 | tee $T/stdout.log"
python3 $S/scripts/suivre-journal.py $T/journal.jsonl --depuis $(date -u +%T)       # attendre l'état sain (≈ 30 s)
```

**[ACTION sur le proxy local, pas sur le cluster] 3a — coupure franche** : `kill $PID`

* Attendu (VÉRIFIÉ à l'identique le 10/11 sur l'ancienne version : « unreachable + tous Unknown 7 s après la coupure ») :
  un cycle `evenement` ou `reprise` où le cluster workload n'est plus `ok` (ligne `CLUSTER my-workload…=unreachable`),
  les dérives qui touchent le workload passent `Unknown` ; des cycles `reprise` à 5, 10, 20, 40, 60 s (plafond), `api` en baisse ;
  le journal du témoin : `Watch des Lease impossible (nouvel essai dans 5 s)`.
* Les dérives qui ne touchent que le management restent `Confirmed`.
* Relancer le proxy (`tmux new -d -s t3-proxy …` avec la même commande) : le prochain cycle `reprise` repasse le cluster à `ok`,
  les verdicts reviennent, et `reprise` s'arrête (premier succès).

**3b — connexion muette** (proxy relancé et état sain revenu) : `kill -STOP $PID`

* Attendu (SUPPOSÉ, non encore observé en vrai) : les connexions restent ouvertes, aucune erreur réseau, mais plus aucune Lease. Environ 30 s
  (θ) + 1 s après la dernière Lease : un cycle **`echeance`** avec `api=0` ; les dérives du workload passent `Unknown` ; celles du management restent
  `Confirmed`. Il n'y a **aucun cycle périodique** avant : tant que les Lease arrivaient, la minuterie était repoussée.
* À surveiller : si un événement du management déclenche un cycle complet pendant le mutisme, la lecture du workload est désormais
  bornée par `--read-timeout` (20 s par défaut, par cluster) : le cycle se termine « partiel » (workload `unreachable`, derniers faits
  conservés), puis une `reprise` à 5, 10, 20… s. L'`echeance` attend au plus ce cycle (un seul cycle à la fois) : le verdict
  `Unknown` doit arriver **au plus ~53 s après la dernière Lease** (31 s si aucun cycle n'est en cours ; SUPPOSÉ, non mesuré en vrai).
  Avant ce réglage le plafond était 30 s par requête et 2 min par cycle. Noter la durée du cycle (`durationMs`) : ~20 s + 2 s attendus.
* `kill -CONT $PID` : les Lease reviennent ; le témoin qui était périmé provoque un rejugement immédiat (`echeance`, `api=0`) et les verdicts
  repassent `Confirmed` sans attendre un autre événement (SUPPOSÉ : testé en unitaire seulement).

**Nettoyage** : `tmux kill-session -t t3-ctl; tmux kill-session -t t3-proxy; tmux kill-session -t t3-ox` (rien n'a changé côté cluster).
