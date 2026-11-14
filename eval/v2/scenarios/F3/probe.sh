#!/bin/bash
# Banc v2 : scenario F3. commit Git invalide
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
[ "$(km -n $KS_NS get kustomization $KS -o jsonpath="{.status.lastAppliedRevision}")" = "$(km -n $KS_NS get gitrepository sylvatwin-poc -o jsonpath="{.status.artifact.revision}")" ] && echo 0 || echo 1
