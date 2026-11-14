#!/bin/bash
# Banc v2 : scenario B1. pod bsf supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] delete pod bsf"; exit 0; fi
mark t0
k -n $NS delete pod -l app.kubernetes.io/name=bsf --wait=false >/dev/null || die "delete pod"
mark inject_fait
