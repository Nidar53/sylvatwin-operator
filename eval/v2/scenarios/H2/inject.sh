#!/bin/bash
# Banc v2 : scenario H2. champ absent de Git modifie (scp.revisionHistoryLimit)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] patch spec.revisionHistoryLimit=3 sur scp"; exit 0; fi
mark t0
orig=$(k -n $NS get deploy scp -o jsonpath='{.spec.revisionHistoryLimit}'); echo "${orig:-10}" > "${RUNDIR:-/tmp}/h2.orig"
k -n $NS patch deploy scp --type merge -p '{"spec":{"revisionHistoryLimit":3}}' >/dev/null || die "patch"
mark inject_fait
