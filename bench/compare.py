#!/usr/bin/env python3
"""Compare two states of rivus on the benchmarks (the engine of bench/compare.sh).

  compare.py run    -rivus BIN -dir DIR -out PREFIX [-rules F] [-ir F] [-timeout 1800s] [-gomemlimit 5GiB]
                    [-wait HOURS] [-resume] [-flags "..."]
  compare.py report -results DIR -base NAME -head NAME -out DIR [-benches a,b,c] [-skipped file]

"run" runs ONE analysis, after the machine is quiet enough (swap use at most 50%, and no other
rivus / rv-* / rivus-* process), and writes PREFIX.json (the report), PREFIX.err and PREFIX.meta.json
(exit code, wall time, peak RSS). With -resume an existing PREFIX.meta.json with exit code 0 is kept.

"report" reads the pairs <bench>.base.* / <bench>.head.* in the results directory and writes
summary.md and details/<bench>.md in -out. Exit status: 0 nothing lost; 1 a finding was removed
(a flow was lost: do not push, classify it); 2 a benchmark did not run or did not finish.
Standard library only.
"""
import argparse, collections, json, os, re, resource, shlex, subprocess, sys, time

SWAP_LIMIT = 0.50
OTHER = re.compile(r"^(rivus|rv-.*|rivus-.*)$")


def swap_fraction():
    """Used / total swap, from `sysctl vm.swapusage` (macOS); None when it cannot be read (Linux: /proc/meminfo)."""
    try:
        out = subprocess.run(["sysctl", "vm.swapusage"], capture_output=True, text=True).stdout
        m = re.search(r"total = ([\d.]+)M\s+used = ([\d.]+)M", out)
        if m and float(m.group(1)) > 0:
            return float(m.group(2)) / float(m.group(1))
    except OSError:
        pass
    try:
        mem = dict((l.split(":")[0], int(l.split()[1])) for l in open("/proc/meminfo"))
        if mem.get("SwapTotal"):
            return 1 - mem["SwapFree"] / mem["SwapTotal"]
    except (OSError, ValueError, IndexError):
        pass
    return None


def other_analyses():
    """Processes that look like another analysis: the command (argv[0]) is rivus, rv-*, rivus-* (not this run's own children)."""
    out = subprocess.run(["ps", "-axo", "pid=,command="], capture_output=True, text=True).stdout
    me = {os.getpid(), os.getppid()}
    found = []
    for line in out.splitlines():
        parts = line.split(None, 1)
        if len(parts) < 2 or int(parts[0]) in me:
            continue
        argv0 = parts[1].split(None, 1)[0]
        if OTHER.match(os.path.basename(argv0)):
            found.append("%s %s" % (parts[0], parts[1][:100]))
    return found


def wait_quiet(hours):
    """Blocks until swap <= 50% and no other analysis runs; returns False when it waited for longer than hours."""
    deadline = time.time() + hours * 3600
    while True:
        sw, others = swap_fraction(), other_analyses()
        if (sw is None or sw <= SWAP_LIMIT) and not others:
            return True
        if time.time() > deadline:
            return False
        why = []
        if sw is not None and sw > SWAP_LIMIT:
            why.append("swap %.0f%% used" % (100 * sw))
        if others:
            why.append("other analysis: " + others[0])
        print("compare: waiting (%s)" % "; ".join(why), file=sys.stderr, flush=True)
        time.sleep(60)


def run(a):
    meta_path = a.out + ".meta.json"
    if a.resume and os.path.exists(meta_path):
        try:
            if json.load(open(meta_path)).get("rc") == 0:
                print("compare: resume, keeping", a.out, file=sys.stderr)
                return 0
        except (OSError, ValueError):
            pass
    if not a.no_guard and not wait_quiet(a.wait):
        json.dump({"rc": 125, "reason": "the machine was not quiet for %s hours" % a.wait}, open(meta_path, "w"))
        print("compare: gave up waiting for a quiet machine", file=sys.stderr)
        return 125
    cmd = [os.path.abspath(a.rivus), "analyze", "-timeout", a.timeout]
    if a.rules:
        cmd += ["-rules", os.path.abspath(a.rules)]
    if a.ir:
        cmd += ["-ir", os.path.abspath(a.ir)]
    cmd += shlex.split(a.flags)
    if not a.ir:
        cmd.append("./...")
    env = dict(os.environ)
    if a.gomemlimit:
        env["GOMEMLIMIT"] = a.gomemlimit
    hard = parse_seconds(a.timeout) * 1.5 + 600
    t0 = time.time()
    with open(a.out + ".json", "wb") as so, open(a.out + ".err", "wb") as se:
        p = subprocess.Popen(cmd, cwd=a.dir, stdout=so, stderr=se, env=env)
        while True:
            pid, status, ru = os.wait4(p.pid, os.WNOHANG)
            if pid:
                break
            if time.time() - t0 > hard:
                p.kill()
            time.sleep(1)
    p.returncode = os.waitstatus_to_exitcode(status)
    wall = time.time() - t0
    rss = ru.ru_maxrss / (1048576 if sys.platform == "darwin" else 1024)  # bytes on macOS, KB on Linux
    meta = {"rc": p.returncode, "wall_s": round(wall, 1), "max_rss_mb": round(rss), "cmd": cmd, "gomemlimit": a.gomemlimit}
    json.dump(meta, open(meta_path, "w"), indent=1)
    print("compare: %s rc=%d %.0fs %d MB" % (os.path.basename(a.out), p.returncode, wall, rss), file=sys.stderr)
    return 0 if p.returncode == 0 else 3


def parse_seconds(s):
    m = re.fullmatch(r"(\d+(?:\.\d+)?)([smh]?)", s)
    if not m:
        return 1800.0
    return float(m.group(1)) * {"": 1, "s": 1, "m": 60, "h": 3600}[m.group(2)]


# ---- report ----

def pos(p):
    return "%s:%s" % (p.get("file", ""), p.get("line", 0))


def key(f):
    return (pos(f["source"]["position"]), pos(f["sink"]["position"]), f["sink"].get("symbol", ""), f.get("category", ""))


def info(f):
    steps = (f.get("paths") or [{}])[0].get("steps") or []
    return {
        "reach": f.get("reachability", ""),
        "len": f.get("path_steps") or len(steps),
        "starts": bool(steps) and steps[0].get("func") == f["source"].get("func"),
        "others": f.get("other_sources_count", len(f.get("other_sources") or [])),
        "src_func": f["source"].get("func", ""),
        "line": "%s (%s) -> %s %s [%s]" % (pos(f["source"]["position"]), f["source"].get("func", ""), pos(f["sink"]["position"]),
                                           f["sink"].get("symbol", ""), f.get("category", "")),
    }


def index(rep):
    d = collections.defaultdict(list)
    for f in rep["findings"]:
        d[key(f)].append(info(f))
    return d


def reasons(rep):
    c = collections.Counter()
    for u in rep.get("uncertainties") or []:
        c[u["kind"]] += 1
        if u["kind"] == "depth_limit":
            c["depth_limit/" + re.split(r"[:\s]", u.get("detail", ""), 1)[0]] += 1
    return c


def load(results, bench, which):
    base = os.path.join(results, "%s.%s" % (bench, which))
    try:
        meta = json.load(open(base + ".meta.json"))
    except (OSError, ValueError):
        return None, None
    if meta.get("rc") != 0:
        return meta, None
    try:
        return meta, json.load(open(base + ".json"))
    except (OSError, ValueError):
        meta["rc"] = 4
        return meta, None


def fmt_delta(b, h):
    return str(b) if b == h else "%s -> %s" % (b, h)


def report(a):
    benches = [b for b in a.benches.split(",") if b]
    skipped = {}
    if a.skipped and os.path.exists(a.skipped):
        for line in open(a.skipped):
            if "\t" in line:
                n, why = line.rstrip("\n").split("\t", 1)
                skipped[n] = why
    os.makedirs(os.path.join(a.out, "details"), exist_ok=True)
    rows, alarms, notes, detail_files = [], [], [], []
    not_run = 0
    removed_total = 0
    for bench in benches:
        if bench in skipped:
            rows.append([bench, "NOT RUN: " + skipped[bench]] + [""] * 9)
            not_run += 1
            continue
        mb, rb = load(a.results, bench, "base")
        mh, rh = load(a.results, bench, "head")
        if rb is None or rh is None:
            why = []
            for w, m, r in (("base", mb, rb), ("head", mh, rh)):
                if r is None:
                    why.append("%s %s" % (w, "did not run" if m is None else "failed (rc=%s%s)" % (m.get("rc"), ", " + m["reason"] if m.get("reason") else "")))
            rows.append([bench, "NOT RUN: " + "; ".join(why)] + [""] * 9)
            not_run += 1
            continue
        ib, ih = index(rb), index(rh)
        added = sorted(k for k in ih if k not in ib)
        removed = sorted(k for k in ib if k not in ih)
        # a key that held n findings and holds fewer is a loss too
        shrunk = sorted(k for k in ib if k in ih and len(ih[k]) < len(ib[k]))
        reach, path = [], []
        for k in sorted(set(ib) & set(ih)):
            b, h = ib[k], ih[k]
            rb_, rh_ = sorted(x["reach"] for x in b), sorted(x["reach"] for x in h)
            if rb_ != rh_:
                reach.append((k, rb_, rh_))
            bb, hh = min(b, key=lambda x: x["len"]), min(h, key=lambda x: x["len"])
            if (bb["len"], bb["starts"]) != (hh["len"], hh["starts"]):
                path.append((k, bb, hh))
        others = sum(1 for k in set(ib) & set(ih) if ib[k][0]["others"] != ih[k][0]["others"])
        ub, uh = reasons(rb), reasons(rh)
        ukinds = sorted(k for k in set(ub) | set(uh) if ub[k] != uh[k])
        sb, sh = rb.get("stats", {}), rh.get("stats", {})
        conv_b, conv_h = sb.get("converged"), sh.get("converged")
        status = []
        if not conv_b or not conv_h:
            status.append("NOT CONVERGED (base %s, head %s): the sets are a weak basis" % (conv_b, conv_h))
        if mb.get("rc") == 0 and (sb.get("timed_out") or sh.get("timed_out")):
            status.append("TIMED OUT")
        if removed or shrunk:
            alarms.append("%s: %d finding(s) REMOVED%s" % (bench, len(removed), " (+%d key(s) with fewer findings)" % len(shrunk) if shrunk else ""))
            removed_total += len(removed) + len(shrunk)
        for s in status:
            notes.append("%s: %s" % (bench, s))
        rows.append([
            bench,
            "!! LOST %d" % (len(removed) + len(shrunk)) if removed or shrunk else "ok",
            fmt_delta(len(rb["findings"]), len(rh["findings"])),
            "+%d" % len(added), "-%d" % len(removed),
            "%d" % len(reach), "%d" % len(path), "%d" % others,
            "%s/%s" % (sb.get("passes"), sh.get("passes")),
            "%s -> %s" % (sb.get("reanalyses"), sh.get("reanalyses")),
            "%s%s" % (("%.0f -> %.0f s" % (mb["wall_s"], mh["wall_s"])), "; %s -> %s MB" % (mb["max_rss_mb"], mh["max_rss_mb"])),
        ])
        if status:
            rows[-1][1] += " | " + "; ".join("NOT CONVERGED" if "CONVERGED" in s else s for s in status)
        # details
        L = ["# %s: %s -> %s" % (bench, a.base, a.head), "",
             "converged: base %s, head %s; passes %s/%s; reanalyses %s -> %s; duration_ms %s -> %s; max RSS %s -> %s MB" % (
                 conv_b, conv_h, sb.get("passes"), sh.get("passes"), sb.get("reanalyses"), sh.get("reanalyses"),
                 sb.get("duration_ms"), sh.get("duration_ms"), mb["max_rss_mb"], mh["max_rss_mb"]),
             "unstable (base / head): %s / %s" % (sb.get("unstable") or [], sh.get("unstable") or []), ""]
        for s in status:
            L += ["**%s**" % s, ""]

        def section(title, items, render):
            L.extend(["## %s (%d)" % (title, len(items)), ""])
            L.extend(render(i) for i in items)
            L.append("")
        section("REMOVED findings (a flow that was lost: classify each)", removed, lambda k: "- !! " + ib[k][0]["line"] + "  [base: %s, path %d steps%s]" % (
            ib[k][0]["reach"], ib[k][0]["len"], "" if ib[k][0]["starts"] else ", path does not start in the source function"))
        section("Keys with fewer findings", shrunk, lambda k: "- !! %s: %d -> %d" % (ib[k][0]["line"], len(ib[k]), len(ih[k])))
        section("ADDED findings", added, lambda k: "- " + ih[k][0]["line"] + "  [head: %s, path %d steps%s]" % (
            ih[k][0]["reach"], ih[k][0]["len"], "" if ih[k][0]["starts"] else ", path does not start in the source function"))
        section("reachability changed", reach, lambda t: "- %s: %s -> %s" % (ib[t[0]][0]["line"], t[1], t[2]))
        section("representative path changed (length, starts in the source function)", path, lambda t: "- %s: %d steps, starts=%s -> %d steps, starts=%s" % (
            t[1]["line"], t[1]["len"], t[1]["starts"], t[2]["len"], t[2]["starts"]))
        L += ["## uncertainties by kind (changed only; depth_limit/<reason>)", ""]
        L += ["- %s: %d -> %d" % (k, ub[k], uh[k]) for k in ukinds] or ["- (none changed)"]
        L.append("")
        open(os.path.join(a.out, "details", bench + ".md"), "w").write("\n".join(L))
        detail_files.append(bench)
        rows[-1].append(ukinds and ", ".join("%s %d->%d" % (k, ub[k], uh[k]) for k in ukinds) or "-")
    out = ["# rivus compare: %s -> %s" % (a.base, a.head), ""]
    for al in alarms:
        out.append("**!! %s**" % al)
    if alarms:
        out += ["", "!! A removed finding is a flow the new state lost. Read each one in details/<bench>.md and classify it "
                "(real / false positive that went away / lost); only a path that is confirmed not to exist is allowed. Do not push.", ""]
    for n in notes:
        out.append("- " + n)
    if not_run:
        out += ["", "!! %d benchmark(s) did not run: by the procedure in bench/README.md, a change that was not measured is not pushed." % not_run]
    out += ["", "| bench | result | findings | added | removed | reachability changed | path changed | other_sources changed | passes b/h | reanalyses | time; peak RSS | uncertainties changed |",
            "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"]
    for r in rows:
        r = r + [""] * (12 - len(r))
        out.append("| " + " | ".join(str(c) for c in r) + " |")
    out += ["", "Details (added / removed / changed findings, one per line): `details/<bench>.md`."]
    open(os.path.join(a.out, "summary.md"), "w").write("\n".join(out) + "\n")
    print("\n".join(out))
    return 1 if removed_total else (2 if not_run else 0)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("run")
    r.add_argument("-rivus", required=True)
    r.add_argument("-dir", required=True)
    r.add_argument("-out", required=True)
    r.add_argument("-rules")
    r.add_argument("-ir")
    r.add_argument("-timeout", default="1800s")
    r.add_argument("-gomemlimit", default="")
    r.add_argument("-wait", type=float, default=6.0, help="hours to wait for a quiet machine")
    r.add_argument("-resume", action="store_true")
    r.add_argument("-no-guard", action="store_true", help="do not wait for a quiet machine (for tests of this script only)")
    r.add_argument("-flags", default="")
    p = sub.add_parser("report")
    p.add_argument("-results", required=True)
    p.add_argument("-base", required=True)
    p.add_argument("-head", required=True)
    p.add_argument("-out", required=True)
    p.add_argument("-benches", required=True)
    p.add_argument("-skipped", help="file of <bench><TAB><reason> for benchmarks that could not be set up")
    a = ap.parse_args()
    sys.exit({"run": run, "report": report}[a.cmd](a))


if __name__ == "__main__":
    main()
