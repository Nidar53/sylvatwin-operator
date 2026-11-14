#!/bin/bash
# Banc v2 : scenario K3. Secret supprime puis pod udm supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] delete secret udm-keys puis delete pod udm"; exit 0; fi
mark t0
k -n $NS delete secret udm-keys || die "delete secret"
k -n $NS delete pod -l app.kubernetes.io/name=udm --wait=false || die "delete pod"
mark inject_fait
