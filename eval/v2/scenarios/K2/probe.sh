#!/bin/bash
# Banc v2 : scenario K2. Secret udm-keys supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
k -n $NS get secret udm-keys >/dev/null 2>&1 && echo 0 || echo 1
