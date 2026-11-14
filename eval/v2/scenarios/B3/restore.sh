#!/bin/bash
# Banc v2 : scenario B3. commit Git legitime replicas udr 2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : git revert puis push"; exit 0; fi
git_revert_push B3 "$REP"
