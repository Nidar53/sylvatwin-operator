#!/bin/bash
# Banc v2 : scenario F1. Kustomization suspendue
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] suspend de la Kustomization sylvatwin-5gc"; exit 0; fi
mark t0
km -n $KS_NS patch kustomization $KS --type merge -p '{"spec":{"suspend":true}}' >/dev/null || die "suspend"
mark inject_fait
