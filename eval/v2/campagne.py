#!/usr/bin/env python3
"""Banc v2 : orchestrateur de campagne (SylvaTwin face a Flux CD). Lance par campagne.sh.
Une seule horloge : celle du serveur (UTC). Aucun jeton, kubeconfig ni contenu de Secret n'est lu, affiche ou archive."""
import argparse, json, os, random, signal, subprocess, sys, threading, time, datetime as dt

V2 = os.path.dirname(os.path.abspath(__file__))
SVC = "/opt/nidar/sylvatwin-run/test2-sain-20261113"
EVRUN = "/opt/nidar/sylvatwin-run/eval-v2"
GITWORK = EVRUN + "/gitwork"
NS, KS_NS, KS = "sylvatwin-5gc", "sylvatwin-flux", "sylvatwin-5gc"
ENV = dict(os.environ)
ENV["NO_PROXY"] = ENV["no_proxy"] = "localhost,127.0.0.1,172.30.0.0/24,100.72.0.0/16,100.73.0.0/16,100.100.0.0/16,100.96.0.0/16,.sylva,.svc,.cluster.local,.cluster.local.,.orange-labs.fr,.intraorange,.francetelecom.fr,.tech.orange"
ENV["PATH"] = "/opt/nidar/go/bin:/opt/nidar/sylva-core/bin:" + ENV.get("PATH", "")
ENV["MGMT_KC"] = "/opt/nidar/sylva-core/management-cluster-kubeconfig"
ENV["WL_KC"] = "/opt/nidar/sylva-core/my-workload-kubeadm-capd-kubeconfig"
ENV["EVRUN"] = EVRUN
WL, MG = ENV["WL_KC"], ENV["MGMT_KC"]

def now(): return dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"
def P(s): return dt.datetime.fromisoformat(s.replace("Z", "+00:00"))
def log(*a): print("[%s]" % now(), *a, flush=True)
def sh(cmd, timeout=60, env=None):
    try:
        r = subprocess.run(cmd, shell=isinstance(cmd, str), capture_output=True, text=True, timeout=timeout, env=env or ENV)
        return r.returncode, r.stdout, r.stderr
    except subprocess.TimeoutExpired:
        return 124, "", "timeout"
def k(*a, t=40): return sh(["kubectl", "--kubeconfig", WL, *a], t)
def km(*a, t=40): return sh(["kubectl", "--kubeconfig", MG, *a], t)
def jk(fn, *a):
    rc, o, e = fn(*a)
    try: return json.loads(o)
    except Exception: return None

# ---------- etat sain ----------
def cycle_state():
    try: d = json.load(open(SVC + "/out/cycle.json"))
    except Exception: return None
    ids = {}
    for l in d.get("driftList", []):
        c, i = l.split(" ", 1); ids[i] = c
    ver = {}; allv = {}
    for l in d.get("verdictList", []):
        v, i = l.split(" ", 1); ver[i] = v; allv.setdefault(i, set()).add(v)
    multi = {i: sorted(v) for i, v in allv.items() if len(v) > 1}   # v2.1 : une derive peut recevoir 2 verdicts (voir DIAGNOSTIC-SUSPENSION)
    return dict(end=d.get("end"), ids=ids, ver=ver, multi=multi, clusters=d.get("clusters", {}), cause=d.get("cause"), trig=d.get("triggerObjects"))

def sain(baseline, allow_label=False):
    """(ok, raisons). Lecture seule."""
    why = []
    pods = jk(k, "-n", NS, "get", "pods", "-o", "json")
    if not pods: return False, ["pods illisibles"]
    its = pods["items"]
    if len(its) != 11: why.append("pods=%d" % len(its))
    for p in its:
        st = p["status"]; n = p["metadata"]["name"]
        if p["metadata"].get("deletionTimestamp"): why.append("Terminating " + n)
        elif st.get("phase") != "Running" or not all(c.get("ready") for c in st.get("containerStatuses", [])): why.append("pod non pret " + n)
    deps = jk(k, "-n", NS, "get", "deploy", "-o", "json")
    for d in (deps or {"items": []})["items"]:
        if d["spec"].get("replicas") != 1 or d["status"].get("readyReplicas") != 1 or d["status"].get("replicas") != 1:
            why.append("deploy %s %s/%s/%s" % (d["metadata"]["name"], d["spec"].get("replicas"), d["status"].get("replicas"), d["status"].get("readyReplicas")))
    ks = jk(km, "-n", KS_NS, "get", "kustomization", KS, "-o", "json")
    gr = jk(km, "-n", KS_NS, "get", "gitrepository", "sylvatwin-poc", "-o", "json")
    if not ks or not gr: return False, ["Flux illisible"]
    cond = {c["type"]: c["status"] for c in ks["status"].get("conditions", [])}
    if cond.get("Ready") != "True": why.append("Kustomization Ready=%s" % cond.get("Ready"))
    if ks["spec"].get("suspend"): why.append("Kustomization suspendue")
    app, art = ks["status"].get("lastAppliedRevision"), gr["status"].get("artifact", {}).get("revision")
    if app != art: why.append("revision appliquee %s != artefact %s" % (str(app)[-12:], str(art)[-12:]))
    rc, head, _ = sh(["git", "-C", GITWORK, "rev-parse", "HEAD"])
    if art and art.split(":")[-1] != head.strip(): why.append("artefact Flux != HEAD du clone")
    if k("-n", NS, "get", "secret", "udm-keys", "-o", "name")[0] != 0: why.append("udm-keys absent")
    nodes = jk(k, "get", "nodes")
    for n in (nodes or {"items": []})["items"]:
        if not allow_label and "sylvatwin.orange.com/zone" in n["metadata"].get("labels", {}): why.append("label de test present sur le noeud")
    for u in ("sylvatwin-oxigraph", "sylvatwin-controller"):
        if sh(["systemctl", "--user", "is-active", u])[1].strip() != "active": why.append("service %s inactif" % u)
    cs = cycle_state()
    if not cs: why.append("cycle.json illisible")
    elif baseline is not None:
        if set(cs["ids"]) != set(baseline): why.append("derives: +%d -%d" % (len(set(cs["ids"]) - set(baseline)), len(set(baseline) - set(cs["ids"]))))
        elif any(v != "ConfirmedViolation" for v in cs["ver"].values()): why.append("verdicts non Confirmed")
        if any(s != "ok" for s in cs["clusters"].values()): why.append("cluster non ok")
    return (not why), why

def wait_sain(baseline, timeout, label="", allow_label=False):
    t0 = time.time(); last = None; okcount = 0
    while time.time() - t0 < timeout:
        ok, why = sain(baseline, allow_label)
        okcount = okcount + 1 if ok else 0
        if okcount >= 2: return True, time.time() - t0     # deux lectures a 10 s d'ecart
        if why != last: log("attente etat sain %s: %s" % (label, "; ".join(why) or "ok (1/2)")); last = why
        time.sleep(10)
    return False, time.time() - t0

# ---------- collecteurs ----------
class Collectors:
    def __init__(self, d):
        self.d = d; self.stop = threading.Event(); self.th = []; self.procs = []
        for sub in ("sylvatwin", "flux", "alerts", "diffs"): os.makedirs(f"{d}/{sub}", exist_ok=True)
        self.lock = threading.Lock(); self.cycles = []
    def start(self):
        for f in (self.sylva, self.fluxpoll, self.fluxevents, self.alerts):
            t = threading.Thread(target=f, daemon=True); t.start(); self.th.append(t)
    def halt(self):
        self.stop.set()
        for p in self.procs:
            try: p.terminate()
            except Exception: pass
        for t in self.th: t.join(timeout=15)
    def sylva(self):
        jf = SVC + "/journal.jsonl"; pos = os.path.getsize(jf); out = open(f"{self.d}/sylvatwin/cycles.jsonl", "a")
        buf = ""
        while not self.stop.is_set():
            try:
                with open(jf) as f:
                    f.seek(pos); chunk = f.read(); pos = f.tell()
            except Exception: chunk = ""
            buf += chunk; lines = buf.split("\n"); buf = lines.pop()
            for l in lines:
                try: d = json.loads(l)
                except Exception: continue
                if "cause" not in d: continue
                time.sleep(0.4); cs = cycle_state()
                rec = dict(t=d["time"], cause=d["cause"], dur=d.get("durationMs"), trig=d.get("triggerObjects"), ids=(cs or {}).get("ids", {}),
                           ver=(cs or {}).get("ver", {}), ver_multi=(cs or {}).get("multi", {}), clusters=d.get("clusters"), cycle_end=(cs or {}).get("end"))
                with self.lock: self.cycles.append(rec)
                out.write(json.dumps(rec) + "\n"); out.flush()
            time.sleep(0.5)
    def last_cycle(self):
        with self.lock: return self.cycles[-1] if self.cycles else None
    def fluxpoll(self):
        out = open(f"{self.d}/flux/poll.jsonl", "a")
        while not self.stop.is_set():
            ks = jk(km, "-n", KS_NS, "get", "kustomization", KS, "-o", "json"); gr = jk(km, "-n", KS_NS, "get", "gitrepository", "sylvatwin-poc", "-o", "json")
            rec = dict(t=now())
            if ks:
                c = {x["type"]: (x["status"], x.get("reason")) for x in ks["status"].get("conditions", [])}
                rec.update(ready=c.get("Ready"), suspend=ks["spec"].get("suspend", False), applied=ks["status"].get("lastAppliedRevision"), attempted=ks["status"].get("lastAttemptedRevision"))
            if gr: rec["artifact"] = gr["status"].get("artifact", {}).get("revision")
            rec["probe"] = self.probe()
            out.write(json.dumps(rec) + "\n"); out.flush()
            self.stop.wait(10)
    probe_cmd = None
    def probe(self):
        if not self.probe_cmd: return None
        rc, o, e = sh(["bash", self.probe_cmd], 30, ENV); return o.strip()
    def fluxevents(self):
        out = open(f"{self.d}/flux/events.log", "a")
        cmd = ["kubectl", "--kubeconfig", MG, "-n", KS_NS, "get", "events", "--watch-only", "--no-headers", "-o",
               "custom-columns=KIND:.involvedObject.kind,NAME:.involvedObject.name,TYPE:.type,REASON:.reason,MSG:.message"]
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, env=ENV); self.procs.append(p)
        for line in p.stdout:
            out.write(now() + "\t" + line.rstrip()[:600] + "\n"); out.flush()
            if self.stop.is_set(): break
    def alerts(self):
        out = open(f"{self.d}/alerts/poll.jsonl", "a")
        while not self.stop.is_set():
            rc, o, e = sh(["kubectl", "--kubeconfig", WL, "get", "--raw", "/api/v1/namespaces/cattle-monitoring-system/services/http:rancher-monitoring-alertmanager:9093/proxy/api/v2/alerts"], 30)
            try:
                al = [dict(name=a["labels"].get("alertname"), ns=a["labels"].get("namespace"), dep=a["labels"].get("deployment") or a["labels"].get("pod"), starts=a.get("startsAt")) for a in json.loads(o)]
            except Exception: al = None
            out.write(json.dumps(dict(t=now(), alerts=al)) + "\n"); out.flush()
            self.stop.wait(10)

def capture_diffs(d, label):
    def clean(txt):
        keep = []
        for l in txt.splitlines():
            if l.startswith(("diff ", "--- ", "+++ ")): continue
            if l[:1] in "+-" and not any(x in l for x in ("generation", "resourceVersion", "managedFields", "uid:", "creationTimestamp", "time:", "manager:", "operation:", "fieldsType", "fieldsV1", "f:")): keep.append(l)
        return "\n".join(keep)[:20000]
    rc, o, e = sh(["kubectl", "--kubeconfig", WL, "diff", "-k", GITWORK + "/usecase/5gc"], 120)
    open(f"{d}/diffs/{label}-kubectl.txt", "w").write(f"rc={rc}\n" + (clean(o) if rc in (0, 1) else e[:1500]))
    rc2, o2, e2 = sh(["flux", "--kubeconfig", MG, "-n", KS_NS, "diff", "kustomization", KS, "--path", GITWORK + "/usecase/5gc"], 150)
    open(f"{d}/diffs/{label}-flux.txt", "w").write(f"rc={rc2}\n" + (clean(o2) if rc2 in (0, 1) else (e2 or o2)[:1500]))

# ---------- une repetition ----------
class Abort(Exception): pass
def run_rep(scen, rep, rundir, baseline, dry=False):
    d = f"{rundir}/{scen}/rep{rep}"; os.makedirs(d, exist_ok=True)
    orc = json.load(open(f"{V2}/scenarios/{scen}/oracle.json"))
    env = dict(ENV, REP=str(rep), RUNDIR=d)
    sd = f"{V2}/scenarios/{scen}"
    if dry:
        for s in [x for x in ("prepare", "inject", "restore") if os.path.exists(f"{sd}/{x}.sh")]:
            rc, o, e = sh(["bash", f"{sd}/{s}.sh", "--dry-run"], 120, env); log(f"[dry-run] {scen} rep{rep} {s}: rc={rc} {o.strip()} {e.strip()[:200]}")
            if rc != 0: raise Abort(f"dry-run {scen}/{s} en echec")
        return
    log(f"=== {scen} rep{rep} : {orc['titre']}")
    ok, dur = wait_sain(baseline, 600, "avant")
    if not ok: raise Abort(f"etat non sain avant {scen} rep{rep}")
    restore_done = False
    def restore():
        nonlocal restore_done
        rc, o, e = sh(["bash", f"{sd}/restore.sh"], 400, env)
        open(f"{d}/restore.log", "a").write(o + e)
        if rc != 0:
            open(f"{EVRUN}/ETAT_BANC.txt", "w").write(f"{now()} restauration de {scen} rep{rep} en echec (rc={rc}). Voir {d}/restore.log. Rien d'autre n'a ete tente.\n")
            raise Abort(f"RESTAURATION EN ECHEC {scen} rep{rep}")
        restore_done = True
    col = Collectors(d); col.probe_cmd = f"{sd}/probe.sh"; col.start(); time.sleep(12)
    marks = f"{d}/marks.tsv"
    injected = False; t0 = None
    gitish = orc["git"] or orc.get("git_prep")
    try:
        if os.path.exists(f"{sd}/prepare.sh"):     # K4 : preparation hors mesure (commit + label), puis retour a un etat stable AVANT la baseline
            injected = True                        # la restauration (idempotente) doit pouvoir defaire la preparation
            rc, o, e = sh(["bash", f"{sd}/prepare.sh"], 900, env); open(f"{d}/prepare.log", "a").write(o + e)
            if rc != 0: raise Abort(f"preparation en echec {scen} rep{rep} : {(o+e)[-300:]}")
            ok, dur = wait_sain(baseline, 600, "apres preparation", allow_label=True)
            if not ok: raise Abort(f"etat non stable apres la preparation de {scen} rep{rep}")
            open(marks, "a").write(f"{now()}\tprep_stable\n")
        open(f"{d}/baseline-avant.json", "w").write(json.dumps(cycle_state()))
        bl = set((cycle_state() or {})["ids"])
        injected = True
        rc, o, e = sh(["bash", f"{sd}/inject.sh"], 300, env); open(f"{d}/inject.log", "a").write(o + e)
        if rc != 0: raise Abort(f"injection en echec {scen} rep{rep} : {(o+e)[-300:]}")
        mk = [l.split("\t") for l in open(marks).read().splitlines()]
        tm = {m[1].split(" ")[0]: m[0] for m in mk}
        t0 = P(tm.get("t0_push_debut") or tm["t0"])
        def diff10():
            time.sleep(max(0, 10 - (dt.datetime.now(dt.timezone.utc) - t0).total_seconds())); capture_diffs(d, "t0+10s")
        dth = threading.Thread(target=diff10); dth.start()
        # attente pilotee par le journal (cycles), avec delai maximal
        seen_new = False; fen = orc["fenetre_s"]
        while True:
            el = (dt.datetime.now(dt.timezone.utc) - t0).total_seconds()
            lc = col.last_cycle(); cur = set(lc["ids"]) if lc and P(lc["t"]) > t0 else None
            if cur is not None and cur - bl: seen_new = True
            if el >= fen: break
            if orc.get("arret_anticipe") == "retour_base" and cur is not None:
                if (seen_new and cur == bl and el > 30) or (orc["benin"] and not seen_new and el >= 75): break
            time.sleep(2)
        dth.join(timeout=300)
        capture_diffs(d, "avant-restauration")
        mark_t = now(); open(marks, "a").write(f"{mark_t}\tfin_attente\n")
        restore()
        open(marks, "a").write(f"{now()}\trestauration_faite\n")
        ok, dur = wait_sain(baseline, 900 if gitish else 600, "apres")
        open(marks, "a").write(f"{now()}\tetat_sain_{'oui' if ok else 'NON'}\n")
        if not ok: raise Abort(f"etat non sain apres {scen} rep{rep}")
        if gitish and not sh(f"git -C {GITWORK} fetch -q origin test && git -C {GITWORK} diff --quiet $(cat {EVRUN}/START_SHA) origin/test -- usecase/5gc")[0] == 0:
            raise Abort(f"./usecase/5gc differe du commit de depart apres {scen} rep{rep}")
        time.sleep(10)
    except Abort as ex:
        if injected and not restore_done and not str(ex).startswith("RESTAURATION"):
            log("abandon -> restauration (idempotente)"); restore()
        raise
    except BaseException as ex:
        log("INTERRUPTION/ERREUR :", repr(ex), "-> restauration")
        if injected and not restore_done: restore()
        raise
    finally:
        col.halt()
        json.dump(dict(scenario=scen, rep=rep, t0=now_s(t0)), open(f"{d}/rep.json", "w"))

def now_s(t): return t.strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z" if t else None

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--reps", type=int, default=3); ap.add_argument("--scenario", action="append"); ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--seed", type=int, default=None); ap.add_argument("--outdir", default=None); ap.add_argument("--skip-prepare", action="store_true")
    a = ap.parse_args()
    allsc = sorted(x for x in os.listdir(V2 + "/scenarios") if os.path.isdir(f"{V2}/scenarios/{x}"))
    sc = a.scenario or allsc
    for s in sc:
        if s not in allsc: sys.exit(f"scenario inconnu {s}")
    seed = a.seed if a.seed is not None else int(time.time()); rnd = random.Random(seed)
    plan = []
    for r in range(1, a.reps + 1):                   # manche par manche : un arret tardif laisse des manches completes
        o = list(sc); rnd.shuffle(o); plan += [(s, r) for s in o]
    rundir = a.outdir or f"{EVRUN}/campagne-{time.strftime('%Y%m%d-%H%M%S', time.gmtime())}{'-dry' if a.dry_run else ''}"
    os.makedirs(rundir, exist_ok=True)
    json.dump(dict(seed=seed, plan=plan, reps=a.reps, started=now(), dry=a.dry_run), open(rundir + "/plan.json", "w"))
    log("campagne", rundir, "graine", seed, "repetitions", len(plan))
    baseline = None
    if os.path.exists(EVRUN + "/baseline-ids.json"): baseline = json.load(open(EVRUN + "/baseline-ids.json"))
    if a.dry_run:
        for s, r in plan:
            run_rep(s, r, rundir, baseline, dry=True)
        log("dry-run complet : OK", len(plan), "repetitions"); return
    if baseline is None: sys.exit("baseline-ids.json absent : lancer prepare.sh d'abord")
    stop = {"v": False}
    def onsig(sig, fr): stop["v"] = True; log("signal", sig, ": arret propre apres la repetition en cours"); 
    signal.signal(signal.SIGTERM, onsig)
    pid = int(sh(["systemctl", "--user", "show", "-p", "MainPID", "--value", "sylvatwin-oxigraph"])[1] or 0)
    rss = lambda: sh(f"grep VmRSS /proc/{pid}/status")[1].split()[1] if pid else "?"
    open(rundir + "/rss.txt", "a").write(f"debut {now()} {rss()} kB\n")
    try:
        for s, r in plan:
            if stop["v"] or os.path.exists(EVRUN + "/STOP"): log("arret demande"); break
            run_rep(s, r, rundir, baseline)
    except Abort as ex:
        log("CAMPAGNE ARRETEE :", ex); open(rundir + "/ARRET.txt", "w").write(f"{now()} {ex}\n"); sys.exit(2)
    finally:
        open(rundir + "/rss.txt", "a").write(f"fin {now()} {rss()} kB\n")
        # journal du kustomize-controller (LECTURE SEULE) : source retenue pour "Flux signale / corrige"
        try:
            since = (P(json.load(open(rundir + "/plan.json"))["started"]) - dt.timedelta(seconds=60)).strftime("%Y-%m-%dT%H:%M:%SZ")
            log("journal kustomize-controller :", sh(["python3", V2 + "/flux_log.py", since, rundir + "/flux-kustomize-controller.jsonl"], 200)[1].strip())
        except Exception as ex: log("journal kustomize-controller non extrait :", repr(ex))
    log("campagne terminee")
if __name__ == "__main__": main()
