#!/bin/bash
# Petite campagne sylvactl (20/11/2026) : 16 scenarios x 2 repetitions, collecteur sylvactl (lecture seule), tmux "sylvactl".
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"; export PYTHONPATH=/opt/nidar/pylib; EV=/opt/nidar/sylvatwin-run/eval-v2; OUT=$EV/campagne-sylvactl-2reps
cd $D
if [ "$1" = reprendre ]; then ./campagne.sh --reprendre --outdir $OUT >> $EV/campagne-sylvactl.log 2>&1
else ./campagne.sh --reps 2 --outdir $OUT > $EV/campagne-sylvactl.log 2>&1; fi
echo "[$(ts)] campagne sylvactl terminee (code $?)" >> $EV/enchainement-sylvactl.log
for o in "" "--fenetre" "--sylvactl"; do python3 $D/analyse.py --version 2.1 $o $OUT > /dev/null 2>&1 || echo "[$(ts)] analyse $o en echec" >> $EV/enchainement-sylvactl.log; done
echo "[$(ts)] analyses ecrites (resultats-v2.1.md, -fenetre.md, -sylvactl.md)" >> $EV/enchainement-sylvactl.log
python3 $D/verif_sain.py --attendre 600 >> $EV/enchainement-sylvactl.log 2>&1 && echo "[$(ts)] etat sain final : OK" >> $EV/enchainement-sylvactl.log
