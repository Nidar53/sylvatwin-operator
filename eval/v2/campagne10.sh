#!/bin/bash
# Campagne a 10 repetitions (16 scenarios, oracle 2.1), detachee (tmux "c10"). Reprise possible : ./campagne10.sh reprendre
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"; export PYTHONPATH=/opt/nidar/pylib; EV=/opt/nidar/sylvatwin-run/eval-v2; OUT=$EV/campagne-v2.3-10reps
cd $D
if [ "$1" = reprendre ]; then ./campagne.sh --reprendre --outdir $OUT >> $EV/campagne-v2.3.log 2>&1
else ./campagne.sh --reps 10 --outdir $OUT > $EV/campagne-v2.3.log 2>&1; fi
echo "[$(ts)] campagne v2.3 (10 reps) terminee (code $?)" >> $EV/enchainement-v23.log
python3 $D/analyse.py --version 2.1 $OUT > /dev/null 2>&1 && python3 $D/analyse.py --version 2.1 --fenetre $OUT > /dev/null 2>&1 && echo "[$(ts)] analyses ecrites (resultats-v2.1.md et resultats-v2.1-fenetre.md)" >> $EV/enchainement-v23.log
python3 $D/verif_sain.py --attendre 600 >> $EV/enchainement-v23.log 2>&1 && echo "[$(ts)] etat sain final : OK" >> $EV/enchainement-v23.log
