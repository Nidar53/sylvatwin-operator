#!/bin/bash
# Banc v2.1 : scenario K4 (placement). PREPARATION, hors mesure (aucune marque t0) :
#  1. commit sur la branche test : nodeSelector sylvatwin.orange.com/zone=eval sur le Deployment bsf (usecase/5gc/bsf.yaml) ;
#  2. attente de l'application par Flux (le nouveau pod bsf reste Pending : aucun noeud n'a encore le label ; l'ancien pod continue de servir) ;
#  3. pose du label de test sur l'unique noeud workload -> le nouveau pod est planifie, l'ancien est supprime ;
#  4. attente de la fin du deploiement (bsf 1/1/1, 11 pods).
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] preparation : commit nodeSelector sur bsf, attente Flux, label $TEST_LABEL=eval sur le noeud workload, attente du deploiement"; exit 0; fi
python3 - "$GITWORK/usecase/5gc/bsf.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old='    spec:\n      automountServiceAccountToken: false\n'
assert s.count(old)==1, "structure bsf.yaml inattendue"
open(p,"w").write(s.replace(old, old+'      nodeSelector: {sylvatwin.orange.com/zone: eval}\n',1))
PY
MARK_PREFIX=prep git_commit_push K4 "$REP" "eval-v2 K4 rép $REP : nodeSelector sur bsf (preparation, a annuler)"
sha=$(git -C $GITWORK rev-parse HEAD)
wait_applied "$sha" 420 || die "Flux n'a pas applique le commit de preparation en 7 min"
mark "prep_flux_applique"
n=$(WL_NODE); case "$n" in my-workload-kubeadm-capd-control-plane-*) ;; *) die "noeud inattendu ($n)";; esac
k label node "$n" "$TEST_LABEL=eval" --overwrite >/dev/null || die "pose du label"
mark "prep_label_pose"
i=0; while [ $i -lt 300 ]; do
  r=$(k -n $NS get deploy bsf -o jsonpath='{.status.replicas}/{.status.updatedReplicas}/{.status.readyReplicas}' 2>/dev/null)
  np=$(k -n $NS get pods --no-headers 2>/dev/null | wc -l)
  [ "$r" = "1/1/1" ] && [ "$np" = 11 ] && break
  sleep 5; i=$((i+5))
done
[ "$r" = "1/1/1" ] || die "bsf non deploye apres la pose du label ($r)"
mark "prep_fin"
