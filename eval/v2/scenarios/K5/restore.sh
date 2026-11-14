#!/bin/bash
# Banc v2 : scenario K5. ConfigMap amf-config modifiee a la main
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : attendre la correction Flux de la ConfigMap, puis rollout restart de amf"; exit 0; fi
for i in $(seq 1 15); do cm_equals_git amf && break; [ $i = 2 ] && flux_reconcile_ks; sleep 10; done
cm_equals_git amf || die "amf-config n'est pas revenue a Git apres 150 s : arret"
k -n $NS rollout restart deploy/amf >/dev/null || die "rollout restart amf"
k -n $NS rollout status deploy/amf --timeout=180s >/dev/null || die "amf ne revient pas"
mark "restaure: amf-config conforme, pod amf redemarre"
