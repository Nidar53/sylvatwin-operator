#!/bin/bash
# Preparation du banc v2 (ETAPE 1). Lecture seule sur les clusters, sauf le redemarrage du service LOCAL sylvatwin-oxigraph.
# Usage : prepare.sh [--sans-redemarrage-oxigraph]
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"
mkdir -p "$EVRUN"
check_ctx
# 1. clone dedie de la branche test (jamais main, jamais le depot de travail)
if [ ! -d "$GITWORK/.git" ]; then
  git clone -q --branch test --single-branch https://github.com/Nidar53/sylvatwin-poc "$GITWORK" || die "clone"
fi
git -C $GITWORK fetch -q origin test; git -C $GITWORK checkout -q test; git -C $GITWORK merge -q --ff-only origin/test || die "le clone n'est pas a jour"
[ -z "$(git -C $GITWORK status --porcelain)" ] || die "clone sale"
applied=$(km -n $KS_NS get kustomization $KS -o jsonpath='{.status.lastAppliedRevision}'); applied=${applied##*:}
head=$(git -C $GITWORK rev-parse HEAD)
[ "$applied" = "$head" ] || die "le HEAD de test ($head) n'est pas la revision appliquee par Flux ($applied)"
[ -f "$EVRUN/START_SHA" ] || echo "$head" > "$EVRUN/START_SHA"
echo "commit de depart : $(cat $EVRUN/START_SHA)"
# 2. redemarrage d'Oxigraph (service local) juste avant la campagne ; history et TBox doivent revenir
if [ "${1:-}" != "--sans-redemarrage-oxigraph" ]; then
  echo "RSS Oxigraph avant : $(grep VmRSS /proc/$(systemctl --user show -p MainPID --value sylvatwin-oxigraph)/status)" | tee -a "$EVRUN/prepare.log"
  systemctl --user restart sylvatwin-oxigraph; date -u +%FT%TZ > "$EVRUN/oxigraph-restart.txt"
fi
Q() { curl -s -H 'Accept: text/csv' --data-urlencode "query=$1" http://127.0.0.1:7878/query; }
for i in $(seq 1 60); do
  tb=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/tbox> { ?s ?p ?o } }' | tail -1 | tr -d '\r')
  hi=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/history> { ?s ?p ?o } }' | tail -1 | tr -d '\r')
  [ "${tb:-0}" -ge 1000 ] 2>/dev/null && [ "${hi:-0}" -ge 40 ] 2>/dev/null && break; sleep 10
done
echo "apres redemarrage : tbox=$tb triplets, history=$hi triplets (attendu ~1247 et ~48)" | tee -a "$EVRUN/prepare.log"
echo "RSS Oxigraph apres : $(grep VmRSS /proc/$(systemctl --user show -p MainPID --value sylvatwin-oxigraph)/status)" | tee -a "$EVRUN/prepare.log"
python3 - <<'PY'
import sys, json, time
sys.path.insert(0, "/opt/nidar/src/sylvatwin-operator/eval/v2")
import campagne as c
ok = False
for i in range(60):
    cs = c.cycle_state()
    ok, why = c.sain(list(cs["ids"]) if cs else None) if cs else (False, ["pas de cycle"])
    # la base est celle de l'etat sain attendu : 5 StaleConfig + 11 FunctionImpact, tous Confirmed
    from collections import Counter
    cnt = Counter(cs["ids"].values()) if cs else {}
    if cs and cnt == Counter({"StaleConfigDrift": 5, "FunctionImpactDrift": 11}) and ok:
        json.dump(sorted(cs["ids"]), open("/opt/nidar/sylvatwin-run/eval-v2/baseline-ids.json", "w"), indent=0)
        print("ETAT SAIN verifie : 5 StaleConfig + 11 FunctionImpact ; base enregistree (cycle", cs["end"], ")"); sys.exit(0)
    print("attente etat sain :", why, dict(cnt)); time.sleep(10)
print("ETAT SAIN NON ATTEINT"); sys.exit(1)
PY
