#!/bin/bash
# Banc v2 : scenario K3. Secret supprime puis pod udm supprime
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
k -n $NS get secret udm-keys >/dev/null 2>&1 && echo 0 || echo 1
