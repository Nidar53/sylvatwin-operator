#!/bin/bash
# Banc v2 : scenario K3. Secret supprime puis pod udm supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : recreation de udm-keys"; exit 0; fi
recreate_udm_keys; mark "restaure: udm-keys present"
