#!/bin/bash
# Banc v2.1 : scenario K4. RESTAURATION (idempotente) : retrait du label (si present) puis git revert du commit de preparation.
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] restauration : retrait du label, git revert du commit de preparation (si present) puis push"; exit 0; fi
n=$(WL_NODE); case "$n" in my-workload-kubeadm-capd-control-plane-*) ;; *) die "noeud inattendu ($n)";; esac
k label node "$n" "$TEST_LABEL-" >/dev/null 2>&1 || true
if git -C $GITWORK log -1 --format=%s | grep -q "^eval-v2 K4 rép $REP : nodeSelector"; then git_revert_push K4 "$REP"; else echo "pas de commit de preparation a annuler"; fi
