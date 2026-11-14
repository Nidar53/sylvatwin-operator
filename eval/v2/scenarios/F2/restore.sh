#!/bin/bash
# Banc v2 : scenario F2. suspension puis scale udr
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : udr a 1 puis reprise"; exit 0; fi
k -n $NS scale deploy/udr --replicas=1 || die "restauration scale udr"
km -n $KS_NS patch kustomization $KS --type merge -p '{"spec":{"suspend":false}}' >/dev/null || die "resume"
mark "restaure: udr a 1, Kustomization reprise"
