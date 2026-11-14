#!/bin/bash
# Banc v2 : scenario F2. suspension puis scale udr
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] suspend puis scale udr a 2"; exit 0; fi
mark t0
km -n $KS_NS patch kustomization $KS --type merge -p '{"spec":{"suspend":true}}' >/dev/null || die "suspend"
mark suspend_fait
sleep 5
k -n $NS scale deploy/udr --replicas=2 || die "scale udr"
mark inject_fait
