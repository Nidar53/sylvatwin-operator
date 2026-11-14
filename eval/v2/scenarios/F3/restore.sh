#!/bin/bash
# Banc v2 : scenario F3. commit Git invalide
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : git revert du commit puis push"; exit 0; fi
git_revert_push F3 "$REP"
