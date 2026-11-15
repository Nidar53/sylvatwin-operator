#!/bin/bash
# Reprise detachee de la campagne v2.2 (tmux) : repetitions restantes, puis analyse et verification de l'etat sain.
D=$(cd "$(dirname "$0")" && pwd); source "$D/lib.sh"
export PYTHONPATH=/opt/nidar/pylib; EV=/opt/nidar/sylvatwin-run/eval-v2
cd $D && ./campagne.sh --reprendre --outdir $EV/campagne-v2.2 >> $EV/campagne-v2.2.log 2>&1
echo "[$(ts)] campagne v2.2 (reprise) terminee (code $?)" >> $EV/enchainement-v22.log
python3 $D/analyse.py --version 2.1 $EV/campagne-v2.2 > /dev/null 2>&1 && echo "[$(ts)] analyse ecrite : $EV/campagne-v2.2/resultats-v2.1.md" >> $EV/enchainement-v22.log
python3 $D/verif_sain.py --attendre 600 >> $EV/enchainement-v22.log 2>&1 && echo "[$(ts)] etat sain final : OK" >> $EV/enchainement-v22.log
