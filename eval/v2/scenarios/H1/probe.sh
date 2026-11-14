#!/bin/bash
# Banc v2 : scenario H1. image changee a la main (scp)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
img=$(k -n $NS get deploy scp -o jsonpath='{.spec.template.spec.containers[?(@.name=="scp")].image}'); [ "$img" = "gradiant/open5gs:2.7.2" ] && echo 0 || echo 1
