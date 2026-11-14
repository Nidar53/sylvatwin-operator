#!/bin/bash
# Banc v2.1 : scenario K4. INJECTION = retrait du label de test du noeud workload : le pod bsf (nodeSelector) tourne sur un noeud
# qui ne satisfait plus son selecteur ; Kubernetes ne l'evince pas (IgnoredDuringExecution) -> PlacementDrift.
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] retrait du label $TEST_LABEL du noeud workload"; exit 0; fi
n=$(WL_NODE); case "$n" in my-workload-kubeadm-capd-control-plane-*) ;; *) die "noeud inattendu ($n)";; esac
mark t0
k label node "$n" "$TEST_LABEL-" >/dev/null || die "retrait du label"
mark inject_fait
