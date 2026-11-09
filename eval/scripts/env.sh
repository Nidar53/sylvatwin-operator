# Environnement des scenarios (source par les autres scripts).
source /opt/nidar/env.sh
# tmux reduit no_proxy a "localhost,127.0.0.1" (constate le 09/11) : les appels
# vers les API CAPD (172.30.0.0/24) partaient alors vers le proxy Orange (407).
# On fixe explicitement la meme valeur que la session interactive.
export NO_PROXY="localhost,127.0.0.1,172.30.0.0/24,100.72.0.0/16,100.73.0.0/16,100.100.0.0/16,100.96.0.0/16,.sylva,.svc,.cluster.local,.cluster.local.,.orange-labs.fr,.intraorange,.francetelecom.fr,.tech.orange"
export no_proxy="$NO_PROXY"
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
