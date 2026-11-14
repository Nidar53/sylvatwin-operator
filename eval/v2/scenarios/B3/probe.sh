#!/bin/bash
# Banc v2 : scenario B3. commit Git legitime replicas udr 2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
[ "$(k -n $NS get deploy udr -o jsonpath="{.spec.replicas}")" = 1 ] && echo 0 || echo 1
