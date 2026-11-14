#!/bin/bash
# Banc v2 : scenario N1. PLMN change en direct dans nrf-config
D=$(cd "$(dirname "$0")/../.." && pwd); source "$D/lib.sh"
check_ctx
if [ "${1:-}" = "--dry-run" ]; then echo "[dry-run] kubectl replace de nrf-config (mnc 70 -> 71)"; exit 0; fi
mark t0
k -n $NS get cm nrf-config -o json | python3 -c '
import sys, json
d = json.load(sys.stdin)
s = d["data"]["nrf.yaml"]
assert "mnc: 70" in s, "mnc 70 absent"
d["data"]["nrf.yaml"] = s.replace("mnc: 70", "mnc: 71")
d["metadata"].pop("managedFields", None)
print(json.dumps(d))' | k replace -f - >/dev/null || die "replace nrf-config"
mark inject_fait
