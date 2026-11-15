#!/usr/bin/env python3
"""Banc memoire Oxigraph (SECONDE instance, port 7979, hors service) : N cycles de faits, RSS toutes les 25 cycles.
Variantes : put (PUT du graphe, comportement actuel) ; drop (DROP SILENT GRAPH puis POST) ; delta (DELETE DATA / INSERT DATA du seul ecart).
Les faits sont ceux d'un instantane reel ; a chaque cycle les horodatages observedAt/lastSyncTime/lastWitnessRenewTime avancent (comme en service)."""
import sys, subprocess, time, urllib.request, urllib.parse, re, os
var, N, port = sys.argv[1], int(sys.argv[2]), 7979
G = "https://w3id.org/sylvatwin/graph/facts"
src = open("/opt/nidar/sylvatwin-run/eval-v2/v22-prep/facts.nt").read()
base = src.splitlines()
TS = re.compile(r'(core#(?:observedAt|lastSyncTime|lastWitnessRenewTime)> ")(\d{4}-\d\d-\d\dT\d\d:\d\d:)(\d\d)(Z")')
UID = re.compile(r'(core#uid> ")([^"]+)(")')
CH = os.environ.get("CHURN") == "1"
def snap(i):
    if CH: return [UID.sub(lambda m: m.group(1) + m.group(2) + "-c%d" % i + m.group(3), l) if "core#uid>" in l else (TS.sub(lambda m: m.group(1) + m.group(2) + "%02d" % ((int(m.group(3)) + i) % 60) + m.group(4), l) if "observedAt>" in l or "Sync" in l or "Witness" in l else l) for l in base]
    return [TS.sub(lambda m: m.group(1) + m.group(2) + "%02d" % ((int(m.group(3)) + i) % 60) + m.group(4), l) if "observedAt>" in l or "Sync" in l or "Witness" in l else l for l in base]
srv = subprocess.Popen(["/opt/nidar/tools/oxigraph", "serve", "--bind", f"127.0.0.1:{port}"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
URL = f"http://127.0.0.1:{port}"
def req(method, path, data=None, ctype=None):
    r = urllib.request.Request(URL + path, data=data, method=method, headers={"Content-Type": ctype} if ctype else {})
    return urllib.request.urlopen(r, timeout=300).read()
for _ in range(50):
    try: req("GET", "/"); break
    except Exception: time.sleep(0.2)
rss = lambda: int(re.search(r"VmRSS:\s+(\d+)", open(f"/proc/{srv.pid}/status").read()).group(1)) // 1024
out = []
prev = None; t0 = time.time()
try:
    for i in range(N):
        cur = snap(i); body = ("\n".join(cur) + "\n").encode()
        if var == "put": req("PUT", "/store?graph=" + urllib.parse.quote(G, safe=""), body, "application/n-triples")
        elif var == "drop":
            req("POST", "/update", ("DROP SILENT GRAPH <%s>" % G).encode(), "application/sparql-update")
            req("POST", "/store?graph=" + urllib.parse.quote(G, safe=""), body, "application/n-triples")
        elif var == "delta":
            if prev is None: req("PUT", "/store?graph=" + urllib.parse.quote(G, safe=""), body, "application/n-triples")
            else:
                sp, sc = set(prev), set(cur); dele = sp - sc; ins = sc - sp
                q = ""
                if dele: q += "DELETE DATA { GRAPH <%s> {\n%s\n} };\n" % (G, "\n".join(dele))
                if ins: q += "INSERT DATA { GRAPH <%s> {\n%s\n} }" % (G, "\n".join(ins))
                req("POST", "/update", q.encode(), "application/sparql-update")
        prev = cur
        req("POST", "/query", urllib.parse.urlencode({"query": "SELECT (COUNT(*) AS ?n) WHERE { GRAPH <%s> { ?s ?p ?o } }" % G}).encode(), "application/x-www-form-urlencoded")
        if i % 25 == 0 or i == N - 1:
            out.append((i, rss())); print(var, i, rss(), "MB", "%.0fs" % (time.time() - t0), flush=True)
finally:
    srv.terminate(); srv.wait()
