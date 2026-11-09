#!/bin/bash
# Attend le retour a l'etat sain (max $1 s, defaut 600). Sortie 1 si echec.
source "$(dirname "$0")/env.sh"
max=${1:-600}; shift; t0=$(date +%s)
while true; do
  out=$("$(dirname "$0")/sain.sh" "$@"); rc=$?
  [ $rc = 0 ] && { echo "$out apres $(( $(date +%s)-t0 )) s"; exit 0; }
  [ $(( $(date +%s)-t0 )) -ge $max ] && { echo "TIMEOUT : $out"; exit 1; }
  sleep 5
done
