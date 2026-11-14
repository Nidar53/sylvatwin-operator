#!/bin/bash
# Banc v2 : scenario N2. tranche ajoutee au SMF dans Git
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
cm_equals_git smf && echo 0 || echo 1
