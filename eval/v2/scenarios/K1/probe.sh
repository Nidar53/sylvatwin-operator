#!/bin/bash
# Banc v2 : scenario K1. scale nrf 1->2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
r=$(k -n $NS get deploy nrf -o jsonpath="{.spec.replicas}"); [ "$r" = 1 ] && echo 0 || echo 1
