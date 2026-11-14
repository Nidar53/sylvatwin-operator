#!/bin/bash
# Banc v2 : scenario F3. commit Git invalide
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] commit sur la branche test (udr.yaml : imagePullPolicy invalide)"; exit 0; fi
python3 - "$GITWORK/usecase/5gc/udr.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old='        - name: udr\n          image: gradiant/open5gs:2.7.2\n'
assert s.count(old)==1, "structure udr.yaml inattendue"
open(p,"w").write(s.replace(old, '        - name: udr\n          image: gradiant/open5gs:2.7.2\n          imagePullPolicy: Bogus\n',1))
PY
git_commit_push F3 "$REP" "eval-v2 F3 rép $REP : imagePullPolicy invalide sur udr (a annuler)"
