#!/bin/bash
# Banc v2 : scenario B4. flux reconcile
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : rien a restaurer"; exit 0; fi
echo "rien a restaurer"
