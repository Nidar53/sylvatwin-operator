#!/bin/bash
# Une REPETITION d'un scenario, de bout en bout.
#   rep.sh <scenario> <n> <duree_observation_s>
# Etapes : etat sain -> historique vide -> boucle SylvaTwin -> injection (heure)
#   -> observation (Flux, kubectl, SylvaTwin) -> annulation -> retour a l'etat
#   sain -> arret de la boucle. Toute la trace va dans eval/runs/<scenario>/<n>/.
# Sortie 3 si le banc ne revient pas a l'etat sain (=> arret de la campagne).
set -u
DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/env.sh"
SC=$1; N=$2; OBS=${3:-240}
export CUR=$RUNS/$SC/$N; rm -rf "$CUR"; mkdir -p "$CUR"
source "$DIR/scenarios.sh"

label_pre=""; label_during=""
case $SC in S1|S3-c) label_pre="a" ;; esac

log "=== $SC repetition $N (observation ${OBS}s, periode ${PERIOD}s)"
"$DIR/attendre_sain.sh" 300 ${label_pre:+--label-attendu $label_pre} | tee -a "$CUR/journal.txt" || { log "ETAT NON SAIN AVANT : arret"; exit 3; }
"$DIR/vider_historique.sh"
km -n $KS_NS get ks $KS -o jsonpath='{.status.lastAppliedRevision}' > "$CUR/revision-avant.txt"
k -n $NS get pods -o wide > "$CUR/pods-avant.txt"

tmux kill-session -t sylvatwin-boucle 2>/dev/null
tmux new-session -d -s sylvatwin-boucle "$DIR/boucle.sh $CUR/cycles"
# attendre un premier cycle complet sur l'etat sain (temoin : doit etre vide)
for i in $(seq 1 60); do ls "$CUR"/cycles/cycle-0001/cycle.json >/dev/null 2>&1 && break; sleep 1; done
pre=$(python3 -c "import json;print(json.load(open('$CUR/cycles/cycle-0001/cycle.json'))['drifts'])" 2>/dev/null)
log "cycle temoin (avant injection) : derives=$pre"

T_INJ_EPOCH=$(date +%s.%N); T_INJ=$(ts)
log "INJECTION"
inject_$SC 2>&1 | tee -a "$CUR/journal.txt"
echo "$T_INJ" > "$CUR/injection.txt"; log "injection terminee a $T_INJ"

# observation : Flux et kubectl a intervalles, SylvaTwin tourne en fond
t_end=$(( $(date +%s) + OBS )); i=0
while [ $(date +%s) -lt $t_end ]; do
  i=$((i+1))
  {
    echo "### $(ts)"
    km -n $KS_NS get ks $KS -o jsonpath='{.status.conditions[?(@.type=="Ready")].status} {.status.conditions[?(@.type=="Ready")].reason} {.status.lastAppliedRevision} suspend={.spec.suspend}{"\n"}'
    k -n $NS get deploy -o custom-columns=N:.metadata.name,SPEC:.spec.replicas,READY:.status.readyReplicas --no-headers | tr '\n' ' '; echo
    k -n $NS get pods --no-headers | awk '{print $1,$2,$3}' | tr '\n' ';'; echo
  } >> "$CUR/observation.txt"
  observe_$SC >> "$CUR/observation.txt" 2>&1
  sleep 10
done
T_OBS_END=$(ts)
# preuves Flux et kubectl (une fois, a la fin de l'observation)
km -n $KS_NS events --for kustomization/$KS > "$CUR/flux-events.txt" 2>&1
km -n flux-system logs deploy/kustomize-controller --since-time="$T_INJ" 2>/dev/null | grep -F '"name":"sylvatwin-5gc"' > "$CUR/flux-kustomize-controller.log"
k -n $NS get events --sort-by=.lastTimestamp > "$CUR/k8s-events.txt" 2>&1
kubectl_view_$SC > "$CUR/kubectl.txt" 2>&1

log "ANNULATION"
undo_$SC 2>&1 | tee -a "$CUR/journal.txt"
"$DIR/attendre_sain.sh" 600 ${label_pre:+--label-attendu $label_pre} | tee -a "$CUR/journal.txt"
rc=${PIPESTATUS[0]}
# laisser un cycle constater la resolution
sleep $(( PERIOD + 10 ))
tmux kill-session -t sylvatwin-boucle 2>/dev/null
k -n $NS get pods -o wide > "$CUR/pods-apres.txt"
km -n $KS_NS get ks $KS -o jsonpath='{.status.lastAppliedRevision}' > "$CUR/revision-apres.txt"
if [ $rc != 0 ]; then log "RETOUR A L'ETAT SAIN : ECHEC"; exit 3; fi
log "retour a l'etat sain : OK"
exit 0
