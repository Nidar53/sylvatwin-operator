#!/bin/bash
# Restauration ponctuelle du banc (perimetre : namespace sylvatwin-5gc) : recree le pod amf perime sans toucher a la spec.
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"; check_ctx
cm_equals_git amf || die "amf-config != Git : rien a faire ici"
for i in 1 2 3; do
  renouveler_pod amf || die "recreation du pod amf"
  [ "$(pod_cree_epoch amf)" -ge "$(cm_modifiee_epoch amf)" ] && { echo "[$(ts)] pod amf recree, plus recent que la ConfigMap"; exit 0; }
  sleep 20
done
die "pod amf toujours plus ancien que la ConfigMap"
