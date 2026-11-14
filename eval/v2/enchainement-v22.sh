#!/bin/bash
# Enchainement automatique (tache detachee, tmux "enchainement") : attend la fin de campagne2, verifie, TESTE, deploie les regles
# corrigees (poc, branche test2), nettoie history (garde les 16 derives ouvertes et leurs dates), redemarre Oxigraph, verifie l'etat sain,
# puis lance la campagne complete v2.2 (3 repetitions, tous les scenarios, oracle 2.1). En cas d'echec : ARRET.txt, rien n'est lance.
# Usage : enchainement-v22.sh [FIX_SHA]
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"
export PYTHONPATH=/opt/nidar/pylib; export PATH=/opt/nidar/go/bin:$PATH
EV=/opt/nidar/sylvatwin-run/eval-v2; ARRET=$EV/ARRET.txt; LOG=$EV/enchainement-v22.log; FIX=${1:-faed569}
POCDEP=/opt/nidar/src/sylvatwin-poc; POCFIX=$EV/poc-fix; OUT=$SVC/out; W=$EV/v22-prep; mkdir -p $W
exec >> $LOG 2>&1
stop() { echo "[$(ts)] ARRET : $*"; echo "$(ts) $*" > $ARRET; echo "Etat : voir $LOG. Aucune campagne lancee." >> $ARRET; exit 2; }
rm -f $ARRET
echo "[$(ts)] enchainement demarre ; attente de la fin de campagne2"
# 1. attendre la fin de campagne2 (session tmux), au plus 6 h
for i in $(seq 1 720); do tmux has-session -t campagne2 2>/dev/null || break; sleep 30; done
tmux has-session -t campagne2 2>/dev/null && stop "campagne2 toujours en cours apres 6 h"
tail -1 $EV/campagne-v2.1-FK4.log | grep -q "campagne terminee" || echo "[$(ts)] ATTENTION : campagne2 ne s'est pas terminee normalement : $(tail -1 $EV/campagne-v2.1-FK4.log)"
[ -f $EV/ETAT_BANC.txt ] && stop "ETAT_BANC.txt present : une restauration a echoue pendant campagne2 ($(cat $EV/ETAT_BANC.txt))"
# 2. etat sain avant toute chose (derives de base seulement ; la campagne2 a pu laisser des derives resolues dans history)
python3 $D/verif_sain.py --attendre 900 || stop "etat non sain apres campagne2"
echo "[$(ts)] etat sain apres campagne2"
# 3. tests AVANT tout deploiement
[ "$(git -C $POCFIX rev-parse --short HEAD)" = "$FIX" ] || stop "poc-fix n'est pas au commit $FIX"
git -C $POCDEP fetch -q origin test2 && [ "$(git -C $POCDEP rev-parse --short origin/test2)" = "$FIX" ] || stop "origin/test2 du poc n'est pas au commit $FIX (correction non poussee ?)"
[ -z "$(git -C $POCDEP status --porcelain)" ] || stop "depot poc deploye sale"
cp $OUT/facts.nt $W/facts.nt; cp $OUT/history.nt $W/history-live.nt
python3 $D/nettoyer_history.py $W/history-live.nt $W/history.nt | tee $W/nettoyage.txt
(cd $POCFIX/tests && python3 verif_v1.py > $W/verif_v1.txt 2>&1) || stop "verif_v1.py (rdflib) en echec : $W/verif_v1.txt"
(cd /opt/nidar/src/sylvatwin-operator && OXIGRAPH_BIN=/opt/nidar/tools/oxigraph SYLVATWIN_POC=$POCFIX SYLVATWIN_POC_OLD=$POCDEP SYLVATWIN_SAIN_DIR=$W \
  timeout 1500 go test ./internal/judge -count=1 -v > $W/go-test-judge.txt 2>&1) || stop "tests Go (conformite rdflib/Oxigraph, reouverture, etat sain) en echec : $W/go-test-judge.txt"
grep -q -- "--- PASS: TestEtatSainInchange" $W/go-test-judge.txt || stop "TestEtatSainInchange non execute"
echo "[$(ts)] tests OK"
# 4. deploiement. Dates des 16 derives ouvertes avant (reference)
grep "core#firstDetectedAt>" $W/history.nt | sort > $W/first-avant.txt
systemctl --user stop sylvatwin-controller || stop "arret du controleur"
cp $OUT/history.nt $EV/history-avant-v22.nt            # archive (rien n'est supprime)
git -C $POCDEP merge -q --ff-only origin/test2 || { systemctl --user start sylvatwin-controller; stop "git merge --ff-only des regles en echec (controleur redemarre, anciennes regles)"; }
[ "$(git -C $POCDEP rev-parse --short HEAD)" = "$FIX" ] && [ -f $POCDEP/rules/lifecycle/00-reouverture-raz.ru ] && [ -f $POCDEP/rules/lifecycle/02b-horizon-raz.ru ] || { systemctl --user start sylvatwin-controller; stop "regles deployees inattendues"; }
cp $W/history.nt $OUT/history.nt                        # history nettoye : derives ouvertes seulement
systemctl --user restart sylvatwin-oxigraph; date -u +%FT%TZ > $EV/oxigraph-restart-v22.txt
echo "RSS Oxigraph apres redemarrage : $(grep VmRSS /proc/$(systemctl --user show -p MainPID --value sylvatwin-oxigraph)/status)" | tee -a $EV/prepare.log
systemctl --user start sylvatwin-controller || stop "demarrage du controleur"
Q() { curl -s -H 'Accept: text/csv' --data-urlencode "query=$1" http://127.0.0.1:7878/query | tail -1 | tr -d '\r'; }
ok=0; for i in $(seq 1 60); do
  tb=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/tbox> { ?s ?p ?o } }'); hi=$(Q 'SELECT (COUNT(*) AS ?n) WHERE { GRAPH <https://w3id.org/sylvatwin/graph/history> { ?s ?p ?o } }')
  [ "${tb:-0}" -ge 1000 ] 2>/dev/null && [ "${hi:-0}" -ge 40 ] 2>/dev/null && { ok=1; break; }; sleep 10; done
[ $ok = 1 ] || stop "TBox/history non rechargees (tbox=$tb history=$hi)"
echo "[$(ts)] apres redemarrage : tbox=$tb history=$hi"
python3 $D/verif_sain.py --attendre 600 --strict || stop "etat non sain (strict) apres deploiement"
sleep 30; python3 $D/verif_sain.py --attendre 120 --strict || stop "etat sain non stable apres deploiement"
grep "core#firstDetectedAt>" $OUT/history.nt | sort > $W/first-apres.txt
python3 - <<PY || stop "les firstDetectedAt des derives ouvertes ont change : voir $W/first-avant.txt / first-apres.txt"
import sys
a=set(open("$W/first-avant.txt")); b=set(open("$W/first-apres.txt"))
sys.exit(0 if a==b and len(a)==len({l.split(" ",1)[0] for l in a}) else 1)
PY
echo "[$(ts)] etat sain strict OK, memes firstDetectedAt ($(wc -l < $W/first-apres.txt) dates)"
# 5. campagne complete v2.2
echo "[$(ts)] lancement de la campagne v2.2"
cd $D && ./campagne.sh --reps 3 --outdir $EV/campagne-v2.2 > $EV/campagne-v2.2.log 2>&1
echo "[$(ts)] campagne v2.2 terminee (code $?)"
python3 $D/analyse.py --version 2.1 $EV/campagne-v2.2 > /dev/null 2>&1 && echo "[$(ts)] analyse ecrite : $EV/campagne-v2.2/resultats-v2.1.md"
python3 $D/verif_sain.py --attendre 600 && echo "[$(ts)] etat sain final : OK"
