#!/usr/bin/env python3
"""Nettoie l'historique avant la campagne v2.2 : ne garde que les derives OUVERTES (sans :resolvedAt) avec toutes leurs dates.
Usage : nettoyer_history.py history.nt sortie.nt   -> imprime "gardees=N supprimees=M"
Les triplets d'une derive resolue (firstDetectedAt cumules, horizon perime, resolvedAt) sont supprimes. Lecture/ecriture de fichiers seulement."""
import sys
src, dst = sys.argv[1:3]
lines = [l for l in open(src) if l.strip()]
resolved = {l.split(" ", 1)[0] for l in lines if "core#resolvedAt>" in l}
keep = [l for l in lines if l.split(" ", 1)[0] not in resolved]
open(dst, "w").write("".join(keep))
subj = lambda ls: {l.split(" ", 1)[0] for l in ls}
print(f"gardees={len(subj(keep))} supprimees={len(resolved)} (triplets {len(keep)}/{len(lines)})")
