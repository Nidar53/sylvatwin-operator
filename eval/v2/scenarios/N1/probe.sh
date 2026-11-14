#!/bin/bash
# Banc v2 : scenario N1. PLMN change en direct dans nrf-config
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
cm_equals_git nrf && echo 0 || echo 1
