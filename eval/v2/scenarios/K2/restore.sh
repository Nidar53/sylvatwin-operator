#!/bin/bash
# Banc v2 : scenario K2. Secret udm-keys supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : recreation de udm-keys (cles d'exemple publiques)"; exit 0; fi
recreate_udm_keys; mark "restaure: udm-keys present"
