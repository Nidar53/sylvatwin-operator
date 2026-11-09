#!/bin/bash
# Campagne : 3 repetitions de chaque scenario + 2 benins. S'arrete au premier
# retour a l'etat sain rate (code 3). Usage : campagne.sh [liste de scenarios]
DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/env.sh"
LISTE=${*:-"S0-A S0-B S2 S3-a S3-b S4-a S5 B1 B2 S4-b S1 S3-c"}
declare -A OBS=( [S0-A]=240 [S3-b]=240 [S0-B]=150 [S2]=120 [S3-a]=120 [S4-a]=240 [S5]=150 [B1]=120 [B2]=150 [S4-b]=120 [S1]=120 [S3-c]=120 )
for sc in $LISTE; do
  if [ "$sc" = S1 ] || [ "$sc" = S3-c ]; then
    if ! k get node $NODE -o jsonpath="{.metadata.labels.sylvatwin\.orange\.com/zone}" | grep -q a; then
      "$DIR/prepare_S1.sh" || { echo "preparation S1 echouee"; exit 3; }
    fi
  fi
  for n in 1 2 3; do
    "$DIR/rep.sh" $sc $n ${OBS[$sc]} || { echo "ARRET CAMPAGNE ($sc rep $n, code $?)"; exit 3; }
  done
done
echo "CAMPAGNE TERMINEE $(ts)"
