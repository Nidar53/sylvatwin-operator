#!/bin/bash
# Banc v2 : scenario N2. tranche ajoutee au SMF dans Git
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
check_git
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] commit sur la branche test (config/smf.yaml : sst 2)"; exit 0; fi
python3 - "$GITWORK/usecase/5gc/config/smf.yaml" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old='    - s_nssai:\n        - sst: 1\n          dnn:\n            - internet\n'
assert s.count(old)==1, "structure config/smf.yaml inattendue"
open(p,"w").write(s.replace(old, '    - s_nssai:\n        - sst: 1\n          dnn:\n            - internet\n        - sst: 2\n          dnn:\n            - internet\n',1))
PY
git_commit_push N2 "$REP" "eval-v2 N2 rép $REP : tranche sst 2 ajoutee au SMF (a annuler)"
