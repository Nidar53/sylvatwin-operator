#!/bin/bash
# Boucle SylvaTwin : un cycle sylvatwin-once, pause PERIOD s, et ainsi de suite.
# Usage : boucle.sh <dossier>   (arret : tmux kill-session -t sylvatwin-boucle)
source "$(dirname "$0")/env.sh"
OUT=$1; mkdir -p "$OUT"; n=0
while true; do
  n=$((n+1)); d=$(printf "%s/cycle-%04d" "$OUT" $n)
  timeout 120 $ONCE --mgmt-kubeconfig $MGMT_KC --rules $POC/rules --sparql $SPARQL --out "$d" > "$d.log" 2>&1
  sleep $PERIOD
done
