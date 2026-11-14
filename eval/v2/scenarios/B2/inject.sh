#!/bin/bash
# Banc v2 : scenario B2. rollout restart udr
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] rollout restart deploy/udr"; exit 0; fi
mark t0
k -n $NS rollout restart deploy/udr >/dev/null || die "rollout restart"
mark inject_fait
