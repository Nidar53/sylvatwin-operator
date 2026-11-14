#!/usr/bin/env python3
"""Verifie l'etat sain du banc (lecture seule) avec les memes criteres que campagne.py. Sortie 0 = sain.
Usage : verif_sain.py [--attendre SECONDES] [--strict]   --strict : exige 5 StaleConfigDrift + 11 FunctionImpactDrift, tous Confirmed, UN verdict par derive, UN firstDetectedAt par derive."""
import sys, json, time
sys.path.insert(0, "/opt/nidar/src/sylvatwin-operator/eval/v2")
import campagne as c
from collections import Counter
wait = int(sys.argv[sys.argv.index("--attendre") + 1]) if "--attendre" in sys.argv else 0
strict = "--strict" in sys.argv
base = json.load(open("/opt/nidar/sylvatwin-run/eval-v2/baseline-ids.json"))
t0 = time.time()
while True:
    ok, why = c.sain(base); cs = c.cycle_state()
    if ok and strict:
        cnt = Counter(cs["ids"].values())
        if cnt != Counter({"StaleConfigDrift": 5, "FunctionImpactDrift": 11}): ok, why = False, [f"derives {dict(cnt)}"]
        elif cs["multi"] or cs["fd_multi"]: ok, why = False, [f"verdicts multiples {cs['multi']} / firstDetectedAt multiples {cs['fd_multi']}"]
    if ok: print("ETAT SAIN"); sys.exit(0)
    if time.time() - t0 >= wait: print("ETAT NON SAIN :", "; ".join(why)); sys.exit(1)
    time.sleep(10)
