#!/bin/bash
# Redemarrage d'Oxigraph ENTRE DEUX MANCHES de la campagne (la memoire d'Oxigraph ne se libere pas : voir docs/MEMOIRE-OXIGRAPH-2026-11-15.md).
# Arrete le controleur, redemarre Oxigraph (en memoire), redemarre le controleur (history rechargee depuis out/history.nt, TBox rechargee au 1er cycle),
# attend TBox + history, puis l'etat sain strict. Echec : code 3 (la campagne s'arrete, ETAT_BANC-style).
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"; export PYTHONPATH=/opt/nidar/pylib
echo "[$(ts)] redemarrage d'Oxigraph : RSS avant $(grep VmRSS /proc/$(systemctl --user show -p MainPID --value sylvatwin-oxigraph)/status | tr -s ' \t' ' ')"
systemctl --user stop sylvatwin-controller || die "arret du controleur"
systemctl --user restart sylvatwin-oxigraph || { systemctl --user start sylvatwin-controller; die "redemarrage d'Oxigraph"; }
echo "[$(ts)] RSS apres $(grep VmRSS /proc/$(systemctl --user show -p MainPID --value sylvatwin-oxigraph)/status | tr -s ' \t' ' ')"
systemctl --user start sylvatwin-controller || die "demarrage du controleur"
Q() { curl -s -H 'Accept: text/csv' --data-urlencode "query=$1" http://127.0.0.1:7878/query | tail -1 | tr -d '\r'; }
ok=0; for i in $(seq 1 60); do
  tb=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/tbox> { ?s ?p ?o } }'); hi=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/history> { ?s ?p ?o } }')
  [ "${tb:-0}" -ge 1000 ] 2>/dev/null && [ "${hi:-0}" -ge 40 ] 2>/dev/null && { ok=1; break; }; sleep 10; done
[ $ok = 1 ] || die "TBox/history non rechargees (tbox=$tb history=$hi)"
python3 $D/verif_sain.py --attendre 600 --strict || die "etat sain strict non retabli apres redemarrage d'Oxigraph"
echo "[$(ts)] Oxigraph redemarre : tbox=$tb history=$hi, etat sain strict OK"
