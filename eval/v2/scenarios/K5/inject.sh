#!/bin/bash
# Banc v2 : scenario K5. ConfigMap amf-config modifiee a la main
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] kubectl replace de amf-config (commentaire ajoute)"; exit 0; fi
mark t0
k -n $NS get cm amf-config -o json | python3 -c '
import sys, json
d = json.load(sys.stdin)
s = d["data"]["amf.yaml"]
d["data"]["amf.yaml"] = s.rstrip("\n") + "\n# eval-v2 K5 : modification a la main\n"
d["metadata"].pop("managedFields", None)
print(json.dumps(d))' | k replace -f - >/dev/null || die "replace amf-config"
mark inject_fait
