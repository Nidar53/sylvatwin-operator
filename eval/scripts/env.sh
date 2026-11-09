# Environnement des scenarios (source par les autres scripts).
source /opt/nidar/env.sh
export NS=sylvatwin-5gc
export KS_NS=sylvatwin-flux KS=sylvatwin-5gc
export NODE=my-workload-kubeadm-capd-control-plane-6w58l
export ZONE=sylvatwin.orange.com/zone
export POC=/opt/nidar/src/sylvatwin-poc
export OP=/opt/nidar/src/sylvatwin-operator
export ONCE=$OP/bin/sylvatwin-once
export RUNS=$OP/eval/runs
export SPARQL=http://127.0.0.1:7878
export PERIOD=15            # secondes entre la fin d'un cycle et le debut du suivant
k()  { kubectl --kubeconfig $WL_KC "$@"; }
km() { kubectl --kubeconfig $MGMT_KC "$@"; }
ts() { date -u +%Y-%m-%dT%H:%M:%S.%3NZ; }
log() { echo "[$(ts)] $*" | tee -a "$CUR/journal.txt"; }
