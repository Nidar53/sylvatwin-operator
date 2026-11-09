#!/bin/bash
# Annule la preparation S1 : revert du commit nodeSelector, Flux applique (pod bsf
# recree sans nodeSelector), PUIS retrait du label. Verifie l'etat sain final.
DIR="$(cd "$(dirname "$0")" && pwd)"; source "$DIR/env.sh"; source "$DIR/scenarios.sh"
export CUR=$RUNS/S1-preparation; mkdir -p $CUR
log "annulation preparation S1"
git -C $POC log -1 --format=%s | grep -q "^TEST S1" || { log "le dernier commit n'est pas le commit de test S1 : arret"; exit 3; }
c=$(git_revert_last); log "revert $c"
flux_reconcile | tee -a $CUR/journal.txt
k -n $NS rollout status deploy/bsf --timeout=180s | tee -a $CUR/journal.txt
k label node $NODE $ZONE- | tee -a $CUR/journal.txt
"$DIR/attendre_sain.sh" 300 | tee -a $CUR/journal.txt
