#!/bin/bash
# Banc v2 : scenario F2. suspension puis scale udr
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
[ "$(km -n $KS_NS get kustomization $KS -o jsonpath="{.spec.suspend}")" = true ] && echo 1 || echo 0
