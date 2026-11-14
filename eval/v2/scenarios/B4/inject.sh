#!/bin/bash
# Banc v2 : scenario B4. flux reconcile
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] flux reconcile kustomization sylvatwin-5gc"; exit 0; fi
mark t0
flux --kubeconfig "$MGMT_KC" -n $KS_NS reconcile kustomization $KS --timeout=3m >/dev/null 2>&1 || die "flux reconcile"
mark inject_fait
