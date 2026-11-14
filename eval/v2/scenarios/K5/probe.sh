#!/bin/bash
# Banc v2 : scenario K5. ConfigMap amf-config modifiee a la main
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
cm_equals_git amf && echo 0 || echo 1
