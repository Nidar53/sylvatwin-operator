#!/bin/bash
# Banc v2 : scenario N1. PLMN change en direct dans nrf-config
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : attendre la correction Flux, sinon flux reconcile"; exit 0; fi
for i in $(seq 1 15); do cm_equals_git nrf && break; [ $i = 2 ] && flux_reconcile_ks; sleep 10; done
cm_equals_git nrf || die "nrf-config n'est pas revenue a Git apres 150 s : arret"
mark "restaure: nrf-config conforme a Git"
