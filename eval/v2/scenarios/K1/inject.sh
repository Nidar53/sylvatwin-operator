#!/bin/bash
# Banc v2 : scenario K1. scale nrf 1->2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] kubectl scale deploy/nrf --replicas=2"; exit 0; fi
mark t0
k -n $NS scale deploy/nrf --replicas=2 || die "scale"
mark inject_fait
