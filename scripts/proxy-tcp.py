#!/usr/bin/env python3
"""Proxy TCP local transparent (test de fraicheur, lecture seule) :
127.0.0.1:19443 -> 172.30.0.2:6443 (control-plane du workload). Le certificat du
control-plane couvre 127.0.0.1 : le TLS reste valide. Ne modifie rien sur le cluster.
  coupure franche : kill <pid>             (connexion refusee)
  connexion muette : kill -STOP <pid>      (connexions ouvertes, plus aucun octet ; kill -CONT <pid> pour reprendre)
Usage : proxy-tcp.py [ecoute=127.0.0.1:19443] [cible=172.30.0.2:6443]"""
import asyncio, os, sys
ecoute = (sys.argv[1] if len(sys.argv) > 1 else '127.0.0.1:19443').split(':')
cible = (sys.argv[2] if len(sys.argv) > 2 else '172.30.0.2:6443').split(':')
async def pipe(r, w):
    try:
        while (b := await r.read(65536)):
            w.write(b); await w.drain()
    except Exception: pass
    finally:
        try: w.close()
        except Exception: pass
async def client(r, w):
    try: tr, tw = await asyncio.open_connection(cible[0], int(cible[1]))
    except Exception: w.close(); return
    await asyncio.gather(pipe(r, tw), pipe(tr, w))
async def main():
    s = await asyncio.start_server(client, ecoute[0], int(ecoute[1]))
    print('proxy pid', os.getpid(), 'sur', ':'.join(ecoute), '->', ':'.join(cible), flush=True)
    async with s: await s.serve_forever()
asyncio.run(main())
