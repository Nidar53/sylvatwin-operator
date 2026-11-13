#!/usr/bin/env python3
"""Suit le journal JSONL du controleur et affiche UNE ligne par cycle :
heure UTC | cause | derives (classe:nombre) | verdicts | duree | requetes API | objet declencheur.
Lecture seule (un fichier). Usage : suivre-journal.py <journal.jsonl> [--depuis HH:MM:SS]"""
import json, sys, time, os
path = sys.argv[1]
depuis = sys.argv[sys.argv.index('--depuis') + 1] if '--depuis' in sys.argv else ''
def ligne(d):
    v = {}
    for cl, m in (d.get('driftsByVerdict') or {}).items():
        for ve, n in m.items():
            v[ve.replace('Violation', '').replace('Verdict', '')] = v.get(ve.replace('Violation', '').replace('Verdict', ''), 0) + n
    dr = ' '.join('%s:%d' % (k.replace('Drift', ''), n) for k, n in sorted((d.get('drifts') or {}).items()))
    api = sum((d.get('apiRequests') or {}).values())
    st = [c.split('/')[-1] + '=' + s for c, s in d.get('clusters', {}).items() if s != 'ok']
    obj = '; '.join((d.get('triggerObjects') or [])[:2])
    return '%s | %-9s | %s | %s | %5d ms | api=%-3d %s| %s' % (
        d['time'][11:19], d['cause'], dr or '-', ' '.join('%s:%d' % kv for kv in sorted(v.items())) or '-',
        d['durationMs'], api, ('| CLUSTER ' + ','.join(st) + ' ') if st else '', obj)
f = open(path)
for l in f:                       # rattrapage (depuis l'heure demandee)
    try: d = json.loads(l)
    except Exception: continue
    if 'cause' in d and d['time'][11:19] >= depuis: print(ligne(d), flush=True)
while True:                       # puis suivi
    l = f.readline()
    if not l: time.sleep(0.5); continue
    try: d = json.loads(l)
    except Exception: continue
    if 'cause' in d: print(ligne(d), flush=True)
