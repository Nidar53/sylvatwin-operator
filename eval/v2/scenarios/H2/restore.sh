#!/bin/bash
# Banc v2 : scenario H2. champ absent de Git modifie (scp.revisionHistoryLimit)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : revisionHistoryLimit remis a la valeur d'origine"; exit 0; fi
o=$(cat "${RUNDIR:-/tmp}/h2.orig" 2>/dev/null || echo 10); k -n $NS patch deploy scp --type merge -p "{\"spec\":{\"revisionHistoryLimit\":$o}}" >/dev/null || die "restauration"; mark "restaure: revisionHistoryLimit=$o"
