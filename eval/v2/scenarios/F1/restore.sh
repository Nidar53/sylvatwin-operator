#!/bin/bash
# Banc v2 : scenario F1. Kustomization suspendue
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : reprise de la Kustomization"; exit 0; fi
km -n $KS_NS patch kustomization $KS --type merge -p '{"spec":{"suspend":false}}' >/dev/null || die "resume"
mark "restaure: Kustomization reprise"
