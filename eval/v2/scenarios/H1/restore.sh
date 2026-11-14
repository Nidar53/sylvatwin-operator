#!/bin/bash
# Banc v2 : scenario H1. image changee a la main (scp)
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : image scp remise a la valeur de Git"; exit 0; fi
img=$(k -n $NS get deploy scp -o jsonpath='{.spec.template.spec.containers[?(@.name=="scp")].image}')
if [ "$img" != "gradiant/open5gs:2.7.2" ]; then k -n $NS set image deploy/scp scp=gradiant/open5gs:2.7.2 >/dev/null || die "restauration image"; k -n $NS rollout status deploy/scp --timeout=180s >/dev/null || die "scp ne revient pas"; mark "restaure: image scp remise"; else echo "deja conforme (Flux)"; fi
