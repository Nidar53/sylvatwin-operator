#!/bin/bash
# Banc v2 : scenario B3. commit Git legitime replicas udr 2
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] commit sur la branche test (udr replicas 2)"; exit 0; fi
python3 - "$GITWORK/usecase/5gc/udr.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old='  replicas: 1\n'
assert s.count(old)==1, "structure udr.yaml inattendue"
open(p,"w").write(s.replace(old, '  replicas: 2\n',1))
PY
git_commit_push B3 "$REP" "eval-v2 B3 rép $REP : replicas de udr 1 -> 2 (a annuler)"
