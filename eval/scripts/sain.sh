#!/bin/bash
# Verifie l'etat sain du banc. Sortie 0 si sain, 1 sinon (avec le detail).
# Usage : sain.sh [--label-attendu a]   (S1/S3-c : le label zone peut etre attendu)
source "$(dirname "$0")/env.sh"
want_label=""; [ "$1" = "--label-attendu" ] && want_label=$2
ok=0; why=()
pods=$(k -n $NS get pods --no-headers 2>/dev/null)
n=$(echo "$pods" | grep -c .)
notready=$(echo "$pods" | awk '$2 !~ /^1\/1$/ || $3 != "Running"' | grep -c .)
[ "$n" = 11 ] && [ "$notready" = 0 ] || { ok=1; why+=("pods: $n au total, $notready non prets"); }
term=$(k -n $NS get pods --no-headers 2>/dev/null | grep -c Terminating); [ "$term" = 0 ] || { ok=1; why+=("pods Terminating: $term"); }
ready=$(km -n $KS_NS get ks $KS -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
susp=$(km -n $KS_NS get ks $KS -o jsonpath='{.spec.suspend}')
app=$(km -n $KS_NS get ks $KS -o jsonpath='{.status.lastAppliedRevision}')
art=$(km -n $KS_NS get gitrepository sylvatwin-poc -o jsonpath='{.status.artifact.revision}')
[ "$ready" = True ] || { ok=1; why+=("Kustomization Ready=$ready"); }
[ "$susp" != true ] || { ok=1; why+=("Kustomization suspendue"); }
[ "$app" = "$art" ] || { ok=1; why+=("revision appliquee $app != artefact $art"); }
k -n $NS get secret udm-keys >/dev/null 2>&1 || { ok=1; why+=("udm-keys absent"); }
lbl=$(k get node $NODE -o jsonpath="{.metadata.labels.sylvatwin\.orange\.com/zone}")
[ "$lbl" = "$want_label" ] || { ok=1; why+=("label zone='$lbl' (attendu '$want_label')"); }
for d in $(k -n $NS get deploy -o jsonpath='{.items[*].metadata.name}'); do
  s=$(k -n $NS get deploy $d -o jsonpath='{.spec.replicas}/{.status.readyReplicas}')
  [ "$s" = "1/1" ] || { ok=1; why+=("deploy $d replicas $s"); }
done
if [ $ok = 0 ]; then echo "SAIN (revision $app)"; else echo "NON SAIN : ${why[*]}"; fi
exit $ok
