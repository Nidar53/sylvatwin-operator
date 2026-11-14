#!/bin/bash
# Banc v2 : scenario K5. ConfigMap amf-config modifiee a la main
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : attendre la correction Flux de la ConfigMap, puis suppression du pod amf (jamais rollout restart : Flux le defait)"; exit 0; fi
for i in $(seq 1 15); do cm_equals_git amf && break; [ $i = 2 ] && flux_reconcile_ks; sleep 10; done
cm_equals_git amf || die "amf-config n'est pas revenue a Git apres 150 s : arret"
# Pod recree par "delete pod" (meme ReplicaSet, spec intacte), puis controle : le pod doit etre plus recent que la ConfigMap ; 3 essais.
ok=0
for essai in 1 2 3; do
  cm_equals_git amf || die "amf-config n'est plus conforme a Git (essai $essai)"
  renouveler_pod amf || die "suppression/recreation du pod amf (essai $essai)"
  if [ "$(pod_cree_epoch amf)" -ge "$(cm_modifiee_epoch amf)" ]; then ok=1; break; fi
  mark "restauration K5 : pod amf plus ancien que la ConfigMap (essai $essai), nouvel essai"; sleep 20
done
[ $ok = 1 ] || die "pod amf toujours plus ancien que amf-config apres 3 essais"
mark "restaure: amf-config conforme, pod amf redemarre"
