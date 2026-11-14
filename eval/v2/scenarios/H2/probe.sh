#!/bin/bash
# Banc v2 : scenario H2. champ absent de Git modifie (scp.revisionHistoryLimit)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
[ "$(k -n $NS get deploy scp -o jsonpath='{.spec.revisionHistoryLimit}')" = 3 ] && echo 1 || echo 0
