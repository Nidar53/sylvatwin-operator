# Bibliotheque du banc v2 (source par les scripts). Une seule horloge : celle du serveur (date -u).
# CADRE : namespace sylvatwin-5gc (workload), Kustomization sylvatwin-flux/sylvatwin-5gc, branche test de
# sylvatwin-poc dans ./usecase/5gc (F3, N2, B3 seulement). Rien d'autre.
source /opt/nidar/env.sh
export NO_PROXY="localhost,127.0.0.1,172.30.0.0/24,100.72.0.0/16,100.73.0.0/16,100.100.0.0/16,100.96.0.0/16,.sylva,.svc,.cluster.local,.cluster.local.,.orange-labs.fr,.intraorange,.francetelecom.fr,.tech.orange"
export no_proxy="$NO_PROXY"
V2=/opt/nidar/src/sylvatwin-operator/eval/v2
export NS=sylvatwin-5gc KS_NS=sylvatwin-flux KS=sylvatwin-5gc
export SVC=/opt/nidar/sylvatwin-run/test2-sain-20261113          # service sylvatwin-controller (journal.jsonl, out/)
export GITWORK=/opt/nidar/sylvatwin-run/eval-v2/gitwork           # clone dedie, branche test
export EVRUN=${EVRUN:-/opt/nidar/sylvatwin-run/eval-v2}
k()  { kubectl --kubeconfig "$WL_KC" "$@"; }
km() { kubectl --kubeconfig "$MGMT_KC" "$@"; }
ts() { date -u +%Y-%m-%dT%H:%M:%S.%3NZ; }
die() { echo "[$(ts)] ERREUR : $*" >&2; exit 3; }
mark() { [ -n "${RUNDIR:-}" ] && printf '%s\t%s\n' "$(ts)" "$*" >> "$RUNDIR/marks.tsv"; echo "[$(ts)] $*"; }
START_SHA() { cat "$EVRUN/START_SHA" 2>/dev/null; }

# Contexte : refuse de continuer si ce n'est pas le bon cluster / namespace / Kustomization.
check_ctx() {
  local n; n=$(k get nodes -o name 2>/dev/null | head -1)
  case "$n" in node/my-workload-kubeadm-capd-control-plane-*) ;; *) die "mauvais cluster workload ($n)";; esac
  k get ns "$NS" -o name >/dev/null 2>&1 || die "namespace $NS absent"
  [ "$(km -n $KS_NS get kustomization $KS -o jsonpath='{.spec.path}' 2>/dev/null)" = "./usecase/5gc" ] || die "Kustomization $KS : chemin inattendu"
  [ "$(km -n $KS_NS get kustomization $KS -o jsonpath='{.spec.sourceRef.name}')" = "sylvatwin-poc" ] || die "sourceRef inattendue"
  [ "$(km -n $KS_NS get gitrepository sylvatwin-poc -o jsonpath='{.spec.ref.branch}')" = "test" ] || die "GitRepository : branche != test"
  km -n $KS_NS get gitrepository sylvatwin-poc -o jsonpath='{.spec.url}' | grep -q 'Nidar53/sylvatwin-poc' || die "GitRepository : depot inattendu"
}
check_git() {
  [ -d "$GITWORK/.git" ] || die "clone $GITWORK absent"
  [ "$(git -C $GITWORK rev-parse --abbrev-ref HEAD)" = "test" ] || die "branche du clone != test"
  git -C $GITWORK remote get-url origin | grep -q 'Nidar53/sylvatwin-poc' || die "origine du clone inattendue"
  [ -z "$(git -C $GITWORK status --porcelain)" ] || die "clone sale"
  local bad; bad=$(git -C $GITWORK diff --name-only "$(START_SHA)" HEAD | grep -v '^usecase/5gc/' || true)
  [ -z "$bad" ] || die "des fichiers hors usecase/5gc ont change depuis le commit de depart : $bad"
}
# Commit d'injection sur la branche test : $1 = scenario, $2 = rep, $3 = fichier (relatif), le fichier est deja modifie.
git_commit_push() {
  local scen=$1 rep=$2 msg=$3
  check_git_dirty_ok
  git -C $GITWORK add -- usecase/5gc || die "git add"
  [ -z "$(git -C $GITWORK diff --cached --name-only | grep -v '^usecase/5gc/')" ] || die "fichier hors usecase/5gc dans le commit"
  git -C $GITWORK commit -q -m "$msg" || die "git commit"
  # MARK_PREFIX vide : marques de mesure (t0_push_debut, push_fait). MARK_PREFIX=prep : commit de PREPARATION (K4), sans toucher a t0.
  local m1="t0_push_debut" m2="push_fait"; [ -n "${MARK_PREFIX:-}" ] && { m1="${MARK_PREFIX}_push_debut"; m2="${MARK_PREFIX}_push_fait"; }
  mark "$m1"
  git -C $GITWORK push -q origin HEAD:test || die "git push (jamais de --force)"
  mark "$m2 $(git -C $GITWORK rev-parse HEAD)"
}
# Attend (max $2 s) que Flux ait applique la revision du commit $1 (sha) : lastAppliedRevision == test@sha1:<sha>.
wait_applied() {
  local sha=$1 max=${2:-420} i=0
  while [ $i -lt $max ]; do
    [ "$(km -n $KS_NS get kustomization $KS -o jsonpath='{.status.lastAppliedRevision}' 2>/dev/null)" = "test@sha1:$sha" ] && return 0
    sleep 5; i=$((i+5))
  done
  return 1
}
WL_NODE() { k get nodes -o jsonpath='{.items[0].metadata.name}'; }
TEST_LABEL=sylvatwin.orange.com/zone
check_git_dirty_ok() { [ "$(git -C $GITWORK rev-parse --abbrev-ref HEAD)" = "test" ] || die "branche != test"; }
git_revert_push() {  # $1 = scenario, $2 = rep : revert du DERNIER commit (qui doit etre le notre)
  local scen=$1 rep=$2
  git -C $GITWORK log -1 --format=%s | grep -q "^eval-v2 $scen rép $rep :" || die "le dernier commit n'est pas celui de $scen rep $rep : restauration refusee"
  git -C $GITWORK revert --no-commit HEAD || die "git revert"
  git -C $GITWORK commit -q -m "eval-v2 $scen rép $rep : restauration (revert)" || die "commit du revert"
  git -C $GITWORK push -q origin HEAD:test || die "push du revert"
  mark "revert_pousse $(git -C $GITWORK rev-parse HEAD)"
}
git_tree_identique() { git -C $GITWORK fetch -q origin test && git -C $GITWORK diff --quiet "$(START_SHA)" origin/test -- usecase/5gc; }

# udm-keys : cles SUCI d'EXEMPLE (publiques) de l'image Open5GS ; jamais affichees.
recreate_udm_keys() {
  k -n $NS get secret udm-keys >/dev/null 2>&1 && return 0
  docker run --rm --network none --entrypoint sh gradiant/open5gs:2.7.2 \
    -c 'cd /opt/open5gs/etc/open5gs/hnet && tar cf - curve25519-1.key secp256r1-2.key' \
  | python3 -c '
import sys, tarfile, io, json, base64
t = tarfile.open(fileobj=io.BytesIO(sys.stdin.buffer.read()))
data = {m.name: base64.b64encode(t.extractfile(m).read()).decode() for m in t.getmembers() if m.isfile()}
print(json.dumps({"apiVersion":"v1","kind":"Secret","type":"Opaque",
  "metadata":{"name":"udm-keys","namespace":"sylvatwin-5gc",
    "labels":{"app.kubernetes.io/part-of":"sylvatwin-5gc","sylvatwin.orange.com/hors-git":"true"}},"data":data}))' \
  | k apply -f - >/dev/null || die "recreation de udm-keys"
}
cm_equals_git() {  # $1 = nom (amf|nrf|smf) : data de la ConfigMap == fichier config/ du clone (a la tete de test)
  python3 - "$1" "$GITWORK/usecase/5gc/config/$1.yaml" <<'PY'
import sys, json, subprocess, os
n, f = sys.argv[1], sys.argv[2]
out = subprocess.run(["kubectl","--kubeconfig",os.environ["WL_KC"],"-n","sylvatwin-5gc","get","cm",n+"-config","-o","json"],capture_output=True,text=True).stdout
live = json.loads(out)["data"][n+".yaml"]
sys.exit(0 if live.strip()==open(f).read().strip() else 1)
PY
}
flux_reconcile_ks() { flux --kubeconfig "$MGMT_KC" -n $KS_NS reconcile kustomization $KS --timeout=3m >/dev/null 2>&1; }
