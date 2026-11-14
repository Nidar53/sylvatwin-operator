#!/bin/bash
# Banc v2 : scenario K1. scale nrf 1->2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : scale nrf a 1 si Flux ne l'a pas fait"; exit 0; fi
r=$(k -n $NS get deploy nrf -o jsonpath="{.spec.replicas}"); if [ "$r" != 1 ]; then k -n $NS scale deploy/nrf --replicas=1 || die "restauration scale"; mark "restaure: replicas nrf remis a 1"; else echo "deja a 1 (Flux)"; fi
