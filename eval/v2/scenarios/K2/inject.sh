#!/bin/bash
# Banc v2 : scenario K2. Secret udm-keys supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] delete secret udm-keys"; exit 0; fi
mark t0
k -n $NS delete secret udm-keys || die "delete secret"
mark inject_fait
