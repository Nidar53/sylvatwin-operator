#!/bin/bash
# Prepare S1/S3-c : (1) label zone=a sur le noeud, (2) nodeSelector zone=a dans
# bsf.yaml (commit de test sur sylvatwin-poc@test), applique par Flux. Ordre
# impose : label AVANT nodeSelector (sinon pod Pending = ConvergenceDrift).
DIR="$(cd "$(dirname "$0")" && pwd)"; source "$DIR/env.sh"; source "$DIR/scenarios.sh"
export CUR=$RUNS/S1-preparation; mkdir -p $CUR
log "preparation S1 : label puis nodeSelector"
k label node $NODE $ZONE=a --overwrite
python3 - "$POC/usecase/5gc/bsf.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old="      automountServiceAccountToken: false\n"
assert s.count(old)==1 and "nodeSelector" not in s
s=s.replace(old, old+"      nodeSelector:\n        sylvatwin.orange.com/zone: a\n",1); open(p,"w").write(s)
PY
c=$(git_push_test "TEST S1 (a annuler) : nodeSelector sylvatwin.orange.com/zone=a sur bsf"); log "commit $c"
flux_reconcile | tee -a $CUR/journal.txt
k -n $NS rollout status deploy/bsf --timeout=180s | tee -a $CUR/journal.txt
"$DIR/attendre_sain.sh" 300 --label-attendu a | tee -a $CUR/journal.txt
