#!/usr/bin/env python3
"""Compare two states of rivus on the benchmarks (the engine of bench/compare.sh).

  compare.py run    -rivus BIN -dir DIR -out PREFIX [-rules F] [-ir F] [-timeout 1800s] [-gomemlimit 5GiB]
                    [-wait HOURS] [-resume] [-flags "..."]
  compare.py irsum  FILE ROOT        (prints the sha256 of an IR, with ROOT replaced by <ROOT>)
  compare.py report -results DIR -base NAME -head NAME -out DIR [-benches a,b,c] [-skipped file]

"run" runs ONE analysis, after the machine is quiet enough (swap use at most 50%, and no other
rivus / rv-* / rivus-* process), and writes PREFIX.json (the report), PREFIX.err and PREFIX.meta.json
(exit code, wall time, peak RSS). With -resume an existing PREFIX.meta.json with exit code 0 is kept.

"report" reads the pairs <bench>.base.* / <bench>.head.* in the results directory and writes
summary.md and details/<bench>.md in -out. Exit status: 0 nothing lost; 1 a finding was removed
(a flow was lost: do not push, classify it); 2 a benchmark did not run, or a run is not a valid basis for the
comparison (R-40): head did not converge while base did, either run timed out, the number of functions
fell or grew by 50% or more, or both runs have no finding at all. A change of 10% to 50% in the number of
functions is a warning. Each condition is one line at the top of summary.md.
Standard library only.
"""
import argparse, collections, glob, hashlib, json, os, re, resource, shlex, shutil, subprocess, sys, tempfile, time

HERE = os.path.dirname(os.path.abspath(__file__))
SWAP_LIMIT = 0.50
FREE_LIMIT = 0.40  # macOS: busy below this free-memory fraction (memory_pressure)
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


def free_memory_fraction():
    """Free memory as a fraction (0..1) from `memory_pressure` (macOS), or None. macOS keeps swap files allocated long
    after the memory is free again, so the swap use alone says "busy" on a machine that is idle."""
    try:
        out = subprocess.run(["memory_pressure"], capture_output=True, text=True, timeout=20).stdout
        m = re.search(r"free percentage:\s*(\d+)%", out)
        if m:
            return int(m.group(1)) / 100.0
    except (OSError, subprocess.SubprocessError):
        pass
    return None


def memory_busy():
    """Returns (busy, why). Where the free memory can be read (macOS), it decides: busy below 40%% free. Otherwise the swap use does."""
    free = free_memory_fraction()
    if free is not None:
        return free < FREE_LIMIT, "free memory %.0f%%" % (100 * free)
    sw = swap_fraction()
    if sw is not None:
        return sw > SWAP_LIMIT, "swap %.0f%% used" % (100 * sw)
    return False, ""


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


LIGHT_FREE_LIMIT = 0.25  # the light tier (one run: 60 s or less, 6 GB or less): busy only below this free-memory fraction


def wait_quiet(hours, light=False):
    """Blocks until the memory is free enough and no other analysis runs; returns False when it waited for longer than hours.
    light: the run is small and may go beside others: only a nearly full memory makes it wait."""
    deadline = time.time() + hours * 3600
    while True:
        busy, mem_why = memory_busy()
        others = other_analyses()
        if light:
            free = free_memory_fraction()
            busy, others = (free is not None and free < LIGHT_FREE_LIMIT), []
        if not busy and not others:
            return True
        if time.time() > deadline:
            return False
        why = []
        if busy:
            why.append(mem_why)
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
    cdir, material = cache_entry(a)
    if cdir and os.path.exists(os.path.join(cdir, "meta.json")):
        # the analysis is deterministic (TestDeterministic*): the same binary, input, rules and arguments give the same result.
        # The material of the key is kept with the result and compared when it is read (a collision or an edit by hand is caught).
        try:
            if json.load(open(os.path.join(cdir, "material.json"))) != material:
                raise ValueError("the material of the cache entry is not the one of this run")
            for ext, name in ((".json", "result.json"), (".err", "err")):
                shutil.copyfile(os.path.join(cdir, name), a.out + ext)
            meta = json.load(open(os.path.join(cdir, "meta.json")))
            meta["cached"] = meta.get("cached_at", "?")
            json.dump(meta, open(meta_path, "w"), indent=1)
            print("compare: %s: cached result (%s), the analysis is skipped" % (os.path.basename(a.out), meta["cached"]), file=sys.stderr)
            return 0
        except (OSError, ValueError) as e:
            print("compare: the cache entry is not used: %s" % e, file=sys.stderr)
    if not a.no_guard and not wait_quiet(a.wait, a.light):
        json.dump({"rc": 125, "reason": "the machine was not quiet for %s hours" % a.wait}, open(meta_path, "w"))
        print("compare: gave up waiting for a quiet machine", file=sys.stderr)
        return 125
    cmd = [os.path.abspath(a.rivus), "analyze", "-timeout", a.timeout]
    if a.rules:
        cmd += ["-rules", os.path.abspath(a.rules)]
    if a.ir:
        cmd += ["-ir", os.path.abspath(a.ir)]
    cmd += shlex.split(a.flags)
    if not a.ir and not any(t.startswith("./") for t in shlex.split(a.flags)):   # a flag list that names packages replaces ./...
        cmd.append("./...")
    env = dict(os.environ)
    if a.gomemlimit:
        env["GOMEMLIMIT"] = a.gomemlimit
    hard = parse_seconds(a.timeout) * 1.5 + 600
    load_start = load_now()
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
    # exit code 4 (exitIncomplete): the output is written, the analysis is not complete (timeout, not converged). Downstream it is
    # a result with stats.timed_out / converged to read (as a timed-out run was before the code existed), so rc is kept 0 and the
    # code is recorded in exit_code; an incomplete run is not stored in the cache.
    incomplete = p.returncode == 4
    meta = {"rc": 0 if incomplete else p.returncode, "exit_code": p.returncode, "wall_s": round(wall, 1), "max_rss_mb": round(rss), "cmd": cmd, "gomemlimit": a.gomemlimit,
            "cpu_s": round(ru.ru_utime + ru.ru_stime, 1), "load_start": load_start, "load_end": load_now(), "peak_how": "main process (ru_maxrss)"}
    json.dump(meta, open(meta_path, "w"), indent=1)
    print("compare: %s rc=%d %.0fs %d MB" % (os.path.basename(a.out), p.returncode, wall, rss), file=sys.stderr)
    if cdir and p.returncode == 0 and not incomplete:
        store_cache(a, cdir, material, meta)
    return 0 if p.returncode in (0, 4) else 3


def sha256_file(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


def cache_entry(a):
    """(directory, material) of the cache entry of this run, or ("", None) when it is not to be cached. The key is made of the
    binary, the state of the target, the rules (their content), every argument and the environment: never the name of a version."""
    if not a.cache_dir or not a.cache_tree:
        return "", None
    tree = json.loads(a.cache_tree)
    if tree.get("dirty"):
        return "", None
    try:
        material = {
            "rivus_sha256": sha256_file(a.rivus),
            "tree": tree,
            "rules_sha256": sha256_file(a.rules) if a.rules else "",
            "timeout": a.timeout, "flags": a.flags, "ir": bool(a.ir), "gomemlimit": a.gomemlimit,
            "env": {k: os.environ.get(k, "") for k in ("GOMAXPROCS", "GOGC", "GOFLAGS", "GOOS", "GOARCH", "GODEBUG")},
        }
    except OSError:
        return "", None
    key = hashlib.sha256(json.dumps(material, sort_keys=True).encode()).hexdigest()[:32]
    return os.path.join(a.cache_dir, key), material


def store_cache(a, cdir, material, meta):
    """Keep a result for the next comparison with the same base. A run that a clock decided (timed out, a function that timed
    out, a budget that ran out) or that did not converge is not deterministic: it is not kept."""
    try:
        st = json.load(open(a.out + ".json")).get("stats", {})
    except (OSError, ValueError):
        return
    if st.get("timed_out") or not st.get("converged") or st.get("func_timeouts") or st.get("budget_exhausted"):
        return
    tmp = cdir + ".tmp%d" % os.getpid()
    try:
        os.makedirs(tmp, exist_ok=True)
        shutil.copyfile(a.out + ".json", os.path.join(tmp, "result.json"))
        shutil.copyfile(a.out + ".err", os.path.join(tmp, "err"))
        json.dump(dict(meta, cached_at=time.strftime("cached %Y-%m-%d %H:%M:%S")), open(os.path.join(tmp, "meta.json"), "w"), indent=1)
        json.dump(material, open(os.path.join(tmp, "material.json"), "w"), indent=1, sort_keys=True)
        shutil.rmtree(cdir, ignore_errors=True)
        os.rename(tmp, cdir)
    except OSError:
        shutil.rmtree(tmp, ignore_errors=True)


def irsum(path, root):
    """sha256 of an IR file with the absolute path of its worktree replaced: the IR holds file:///<worktree>/... ids,
    and the same input analysed in two worktrees is the same IR."""
    import hashlib
    h = hashlib.sha256()
    needle, rep = root.rstrip("/").encode(), b"<ROOT>"
    tail = b""
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1 << 22)
            data = tail + chunk
            if not chunk:
                h.update(data.replace(needle, rep))
                break
            cut = max(0, len(data) - len(needle))  # keep a tail so a path split between two chunks is still replaced
            h.update(data[:cut].replace(needle, rep))
            tail = data[cut:]
    return h.hexdigest()


def load_prov(results, bench, which):
    """The line compare.sh wrote for one side: input, extractor, IR sha256, version, rules (TSV)."""
    try:
        f = open(os.path.join(results, "%s.%s.prov" % (bench, which))).read().rstrip("\n").split("\t")
        return dict(zip(("input", "extractor", "ir", "version", "rules"), f))
    except OSError:
        return None


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


def covered(rep):
    """The keys of the flows that a finding lists in other_sources: a flow whose source is a parameter
    can be folded into the finding of another source that reaches the same sink (mergeParamSources),
    and is not lost."""
    out = set()
    for f in rep["findings"]:
        for o in f.get("other_sources") or []:
            out.add((pos(o["position"]), pos(f["sink"]["position"]), f["sink"].get("symbol", ""), f.get("category", "")))
    return out


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


FUNC_WARN, FUNC_FAIL = 0.10, 0.50
NOISE_RATIO, NOISE_MIN = 0.20, 5  # a benchmark without a truth cannot say that an added finding is wrong: a surge of them needs a reading


def noise_check(bench, n_base, n_added):
    """More than 20% of the findings of base, and more than 5, added at once (exit status 2, not 1: nothing was lost, but the
    additions are to be classified one by one, like a removal). There is no flag that silences it: write the classification
    in the commit message or the review."""
    if n_added > NOISE_MIN and n_added > NOISE_RATIO * n_base:
        return "NOISE: %s +%d (%.0f%%): classification needed (read details/%s.md, one by one)" % (bench, n_added, 100.0 * n_added / max(n_base, 1), bench)
    return ""


def check_validity(bench, sb, sh, nb, nh):
    """R-40: what makes a pair of runs unfit to be compared. Returns (invalid, warnings): lines for summary.md."""
    bad, warn = [], []
    if sb.get("converged") and not sh.get("converged"):
        bad.append("%s: head did not converge (base did)" % bench)
    for w, st in (("base", sb), ("head", sh)):
        if st.get("timed_out"):
            bad.append("%s: %s timed out (a clock stopped it: the result depends on the load of the machine)" % (bench, w))
    fb, fh = sb.get("functions"), sh.get("functions")
    if isinstance(fb, int) and isinstance(fh, int) and fb > 0:
        change = abs(fh - fb) / fb
        if change >= FUNC_FAIL:
            bad.append("%s: stats.functions %d -> %d (%.0f%%): the program that was analysed is not the same" % (bench, fb, fh, 100 * change))
        elif change >= FUNC_WARN:
            warn.append("%s: stats.functions %d -> %d (%.0f%%)" % (bench, fb, fh, 100 * change))
    elif isinstance(fb, int) and fb > 0 and fh is None:
        bad.append("%s: head has no stats.functions" % bench)
    if nb == 0 and nh == 0:
        bad.append("%s: no finding in base and none in head (a benchmark with a truth has findings: is the input or the rules empty?)" % bench)
    return bad, warn


COST_MARK = 0.10  # CPU time or peak memory up by this much or more is marked "!!"


def load_now():
    """The 1-minute load average (the time of a run depends on it); None where it cannot be read."""
    try:
        return round(os.getloadavg()[0], 2)
    except (OSError, AttributeError):
        return None


def pct(b, h):
    """The change from b to h in percent, as a string with a sign ("+12%"); "n/a" when either is missing or b is 0."""
    if b is None or h is None or not b:
        return "n/a"
    return "%+.0f%%" % (100.0 * (h - b) / b)


def median(xs):
    xs = sorted(xs)
    n = len(xs)
    return None if not n else (xs[n // 2] if n % 2 else (xs[n // 2 - 1] + xs[n // 2]) / 2.0)


def spread(xs):
    """The spread between runs of the SAME version, relative to their median: (max - min) / median; None for fewer than 2 runs."""
    if len(xs) < 2 or not median(xs):
        return None
    return (max(xs) - min(xs)) / median(xs)


def delta_of(xb, xh):
    """(delta, spread, conclusion): delta = change of the medians; spread = the larger spread of the two sides; the change is a
    conclusion only when both sides have at least 2 runs. marked = delta >= COST_MARK and delta > spread."""
    if not xb or not xh or not median(xb):
        return None, None, False
    d = (median(xh) - median(xb)) / median(xb)
    sp = [x for x in (spread(xb), spread(xh)) if x is not None]
    if len(xb) < 2 or len(xh) < 2:
        return d, None, False
    s_ = max(sp)
    return d, s_, d >= COST_MARK - 1e-9 and d > s_


def marked(xb_cpu, xh_cpu, xb_rss, xh_rss):
    """True when CPU time or peak memory went up by COST_MARK (10%) or more AND by more than the spread between runs of the same
    version (lists of runs; fewer than 2 runs on a side: never marked)."""
    return delta_of(xb_cpu, xh_cpu)[2] or delta_of(xb_rss, xh_rss)[2]


def runs_of(metas, key):
    return [m[key] for m in metas if m.get(key) is not None]


def range_text(xs, fmt):
    if not xs:
        return "n/a"
    return fmt % min(xs) if min(xs) == max(xs) else (fmt + "-" + fmt) % (min(xs), max(xs))


INCOMPLETE_EXIT = 4  # exit code of `rivus analyze` for an analysis that is not complete (timeout, not converged): see run_one
INCOMPLETE_NOTE = "incomplete run: no point comparing"


def incomplete_sides(runs_b, runs_h):
    """The sides ("base", "head") that have a run with exit code 4: it stopped before it was done, so its time and memory are not those
    of an analysis that finished, and a comparison with a complete run says nothing."""
    return [w for w, ms in (("base", runs_b), ("head", runs_h)) if any(m.get("exit_code") == INCOMPLETE_EXIT for m in ms)]


def cost_cell(xb, xh, fmt, incomplete=False):
    """"b-range -> h-range (+12%, spread 3%)"; a single run on a side says "single run: no conclusion" and has no mark. With
    incomplete (a run that did not finish is on a side) the numbers are shown and the change is not: "no point comparing"."""
    if not xb or not xh:
        return "n/a"
    if incomplete:
        return "%s -> %s (%s)" % (range_text(xb, fmt), range_text(xh, fmt), INCOMPLETE_NOTE)
    d, sp, _ = delta_of(xb, xh)
    tail = "single run: no conclusion" if sp is None else "%s, spread %.0f%%" % (pct(median(xb), median(xh)), 100 * sp)
    return "%s -> %s (%s)" % (range_text(xb, fmt), range_text(xh, fmt), tail)


def cost_row(name, runs_b, runs_h, sb, sh):
    """One row of the cost table. runs_b / runs_h: the meta of each run of the side (cpu_s, max_rss_mb, wall_s, load_start, load_end),
    in the order they were run; sb / sh: the stats (deterministic: the elements and the steps are the same in every run).
    Columns: bench, runs, CPU s, peak MB, summary elements, summary steps, mark, wall, load average (first start / last end)."""
    st = lambda d, k: (d or {}).get(k)

    def pair(b, h):
        return "n/a" if b is None and h is None else "%s -> %s (%s)" % ("n/a" if b is None else b, "n/a" if h is None else h, pct(b, h))

    def load(ms):
        a = ms[0].get("load_start") if ms else None
        z = ms[-1].get("load_end") if ms else None
        return "%s/%s" % ("n/a" if a is None else a, "n/a" if z is None else z)
    cpu_b, cpu_h = runs_of(runs_b, "cpu_s"), runs_of(runs_h, "cpu_s")
    rss_b, rss_h = runs_of(runs_b, "max_rss_mb"), runs_of(runs_h, "max_rss_mb")
    n = "%d/%d" % (len(runs_b), len(runs_h))
    inc = incomplete_sides(runs_b, runs_h)
    return [name, n, cost_cell(cpu_b, cpu_h, "%.1f", bool(inc)), cost_cell(rss_b, rss_h, "%d", bool(inc)),
            pair(st(sb, "summary_elems"), st(sh, "summary_elems")), pair(st(sb, "summary_steps"), st(sh, "summary_steps")),
            "!! incomplete (%s)" % ", ".join(inc) if inc else ("!!" if marked(cpu_b, cpu_h, rss_b, rss_h) else ""),
            cost_cell(runs_of(runs_b, "wall_s"), runs_of(runs_h, "wall_s"), "%.0f", bool(inc)),
            "base %s, head %s" % (load(runs_b), load(runs_h))]


COST_HEADER = ["| bench | runs b/h | CPU s (user+sys; min-max) | peak MB | summary elements | summary steps | !! | wall s | load average (first start/last end) |",
               "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"]


def cost_table(rows):
    """The cost table as lines; empty when there are no rows."""
    if not rows:
        return []
    return ["", "Cost, before -> after, runs ALTERNATING base, head, base, head (CPU time is the main number, wall is load dependent). `!!` = CPU or peak memory "
            "+%d%% or more AND more than the spread between runs of the same version; one run on a side: \"single run: no conclusion\", no mark. "
            "A cached base run is not a sample (another time, another load). `!! incomplete` = a run of that side stopped before it was done "
            "(exit code 4: timeout or not converged): its time and memory are not those of a finished analysis, so they are not compared." % round(COST_MARK * 100), ""] + COST_HEADER + \
           ["| " + " | ".join(str(c) for c in r) + " |" for r in rows]


def costcheck(a):
    """One run of a target of bench/deps-targets.json on each side with the default flags: cost numbers only (no finding comparison).
    Written to <results>/cost-<target>.json. The tree is fetched (never run), the run has the watchdog of bench/deps_bench.py:
    its own process group, killed above the memory cap."""
    sys.path.insert(0, HERE)
    import deps_bench as db
    targets = json.load(open(db.TARGETS_FILE))
    t = targets[a.target]
    cache = db.cache_dir()
    limit = a.limit or t["time_limit_s"] * 10
    cap = int((a.cap or t["mem_cap_gb"]) * db.GB)
    with tempfile.TemporaryDirectory(prefix="costcheck-") as work:
        env = dict(os.environ)
        tree = db.fetch(a.target, t, t["commit"], cache)
        src, env, err = db.prepare(a.target, t, tree, work, env)
        if err:
            print("compare: costcheck %s: %s" % (a.target, err), file=sys.stderr)
            return 2
        out = {"base": {"runs": [], "stats": {}}, "head": {"runs": [], "stats": {}}}
        for rnd in range(a.rounds):   # ALTERNATING: base, head, base, head
            for side, binary in (("base", a.base_bin), ("head", a.head_bin)):
                load0 = load_now()
                r = db.analyze_once(os.path.abspath(binary), t, src, env, limit, cap, work, "%s%d" % (side, rnd))
                st = (r["report"] or {}).get("stats", {})
                out[side]["runs"].append({"cpu_s": round(r["cpu"], 1), "max_rss_mb": round(r["peak"] / (1 << 20)), "wall_s": round(r["wall"], 1),
                                          "load_start": load0, "load_end": load_now(), "why": r["why"], "converged": r["converged"]})
                out[side]["stats"] = {"summary_elems": st.get("summary_elems"), "summary_steps": st.get("summary_steps")}
                print("compare: costcheck %s %s round %d: cpu %.0fs, peak %d MB (process-tree sum), wall %.0fs%s" % (
                    a.target, side, rnd + 1, r["cpu"], r["peak"] >> 20, r["wall"], " (stopped: %s)" % r["why"] if r["why"] else ""), file=sys.stderr)
    json.dump(out, open(os.path.join(a.results, "cost-%s.json" % a.target), "w"), indent=1)
    return 0


def cost_runs(results, bench, which, main_meta):
    """The runs of one side that are samples of its cost: the main run (not a cached one: that was measured at another time) and the extra
    rounds <bench>.<which>.r<N>.meta.json (rc 0), in the order of the rounds."""
    runs = []
    if main_meta and main_meta.get("rc") == 0 and not main_meta.get("cached"):
        runs.append(main_meta)
    extra = []
    for f in glob.glob(os.path.join(results, "%s.%s.r*.meta.json" % (bench, which))):
        m = re.search(r"\.r(\d+)\.meta\.json$", f)
        try:
            meta = json.load(open(f))
        except (OSError, ValueError):
            continue
        if m and meta.get("rc") == 0:
            extra.append((int(m.group(1)), meta))
    return runs + [m for _, m in sorted(extra, key=lambda x: x[0])]


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
    cost_rows = []
    alarms_invalid, warnings, cached, alarms_noise = [], [], [], []
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
        for w, m in (("base", mb), ("head", mh)):
            if m.get("cached"):
                cached.append("%s: %s: %s (the result of an earlier run with the same binary, input, rules and arguments; the time, the peak RSS and, while stats.functions is not stable between runs, the stats are of that run)" % (bench, w, m["cached"]))
        ib, ih = index(rb), index(rh)
        added = sorted(k for k in ih if k not in ib)
        folded = covered(rh)
        removed = sorted(k for k in ib if k not in ih and k not in folded)
        merged = sorted(k for k in ib if k not in ih and k in folded)
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
        invalid, warned = check_validity(bench, sb, sh, len(rb["findings"]), len(rh["findings"]))
        for line in invalid:
            alarms_invalid.append(line)
        noise = noise_check(bench, len(ib), len(added))
        if noise:
            alarms_noise.append(noise)
        for line in warned:
            warnings.append(line)
        if removed or shrunk:
            alarms.append("%s: %d finding(s) REMOVED%s" % (bench, len(removed), " (+%d key(s) with fewer findings)" % len(shrunk) if shrunk else ""))
            removed_total += len(removed) + len(shrunk)
        for s in status:
            notes.append("%s: %s" % (bench, s))
        rows.append([
            bench,
            "!! LOST %d" % (len(removed) + len(shrunk)) if removed or shrunk else ("!! INVALID" if invalid else "ok"),
            fmt_delta(len(rb["findings"]), len(rh["findings"])),
            "+%d" % len(added), "-%d" % len(removed),
            "%d" % len(reach), "%d" % len(path), "%d" % others,
            "%s/%s" % (sb.get("passes"), sh.get("passes")),
            "%s -> %s" % (sb.get("reanalyses"), sh.get("reanalyses")),
            "%s%s" % (("%.0f -> %.0f s" % (mb["wall_s"], mh["wall_s"])), "; %s -> %s MB" % (mb["max_rss_mb"], mh["max_rss_mb"])),
        ])
        cost_rows.append(cost_row(bench, cost_runs(a.results, bench, "base", mb), cost_runs(a.results, bench, "head", mh), sb, sh))
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
        inc = [w for w, m in (("base", mb), ("head", mh)) if m.get("exit_code") == INCOMPLETE_EXIT]
        if inc:
            L += ["**!! incomplete (%s): the run stopped before it was done (exit code 4). Its time and memory (max RSS above) are not those of a finished "
                  "analysis: do not compare them with the other side.**" % ", ".join(inc), ""]

        def section(title, items, render):
            L.extend(["## %s (%d)" % (title, len(items)), ""])
            L.extend(render(i) for i in items)
            L.append("")
        section("REMOVED findings (a flow that was lost: classify each)", removed, lambda k: "- !! " + ib[k][0]["line"] + "  [base: %s, path %d steps%s]" % (
            ib[k][0]["reach"], ib[k][0]["len"], "" if ib[k][0]["starts"] else ", path does not start in the source function"))
        section("Folded into the other_sources of another finding (not lost)", merged, lambda k: "- " + ib[k][0]["line"])
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
    for al in alarms_noise:
        out.append("**!! %s**" % al)
    for al in alarms_invalid:
        out.append("**!! INVALID: %s**" % al)
    for w in warnings:
        out.append("warning: %s" % w)
    for c in cached:
        out.append("- %s" % c)
    if alarms_invalid:
        out += ["", "!! The run(s) above are not a valid basis for the comparison (exit status 2): a head that did not finish, or a result "
                "that shrank to nothing, shows no removed finding because there is nothing left to remove. Find out why before reading the table.", ""]
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
    for cf in sorted(glob.glob(os.path.join(a.results, "cost-*.json"))):
        try:
            c = json.load(open(cf))
            name = os.path.basename(cf)[len("cost-"):-len(".json")] + " (cost check; peak = process-tree sum)"
            cost_rows.append(cost_row(name, c["base"]["runs"], c["head"]["runs"], c["base"].get("stats"), c["head"].get("stats")))
            for w in ("base", "head"):
                for r_ in c[w]["runs"]:
                    if r_.get("why"):
                        out.append("warning: cost check %s %s was stopped (%s): its numbers are partial" % (name, w, r_["why"]))
        except (OSError, ValueError, KeyError):
            continue
    out += cost_table(cost_rows)
    prov = []
    for bench in benches:
        pb, ph = load_prov(a.results, bench, "base"), load_prov(a.results, bench, "head")
        if not pb or not ph:
            continue
        same_ir = "same" if pb["ir"] == ph["ir"] else "DIFFERENT"
        if pb["ir"] == "-":
            same_ir = "-"
        prov.append("| %s | %s | %s | %s | %s | %s | %s |" % (
            bench, pb["input"], ph["input"] if ph["input"] != pb["input"] else "(same)",
            pb["extractor"] + ("" if pb["extractor"] == ph["extractor"] else " -> " + ph["extractor"]),
            pb["ir"][:16], ph["ir"][:16], same_ir))
    if prov:
        out += ["", "Inputs of each side (its own version's fixtures and extractor; the IR sha256 has the worktree path removed; a Go target has no IR):", "",
                "| bench | input (base) | input (head) | extractor (tree) | IR sha256 base | IR sha256 head | IR |",
                "| --- | --- | --- | --- | --- | --- | --- |"] + prov
    out += ["", "Details (added / removed / changed findings, one per line): `details/<bench>.md`."]
    open(os.path.join(a.out, "summary.md"), "w").write("\n".join(out) + "\n")
    print("\n".join(out))
    return 1 if removed_total else (2 if not_run or alarms_invalid or alarms_noise else 0)


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
    r.add_argument("-light", action="store_true", help="a small run: it does not wait for other analyses, only for free memory")
    r.add_argument("-cache-dir", default="", help="where results of deterministic runs are kept (with -cache-tree)")
    r.add_argument("-cache-tree", default="", help="JSON, the state of the target: commit, dirty, go.sum, go version, IR sha256 ...")
    sub.add_parser("freemem")
    i = sub.add_parser("irsum")
    i.add_argument("file")
    i.add_argument("root")
    p = sub.add_parser("report")
    p.add_argument("-results", required=True)
    p.add_argument("-base", required=True)
    p.add_argument("-head", required=True)
    p.add_argument("-out", required=True)
    p.add_argument("-benches", required=True)
    p.add_argument("-skipped", help="file of <bench><TAB><reason> for benchmarks that could not be set up")
    c = sub.add_parser("costcheck")
    c.add_argument("-results", required=True)
    c.add_argument("-target", default="memos", help="a target of bench/deps-targets.json")
    c.add_argument("-base-bin", required=True)
    c.add_argument("-head-bin", required=True)
    c.add_argument("-rounds", type=int, default=2, help="alternating rounds (base, head, base, head ...); each run takes minutes")
    c.add_argument("-limit", type=float, default=0, help="seconds (default: 10 times the time limit of the pass line)")
    c.add_argument("-cap", type=float, default=0, help="GB (default: the cap of the pass line, 8)")
    a = ap.parse_args()
    if a.cmd == "freemem":
        f = free_memory_fraction()
        print("" if f is None else int(100 * f))
        return
    if a.cmd == "irsum":
        print(irsum(a.file, a.root))
        return
    sys.exit({"run": run, "report": report, "costcheck": costcheck}[a.cmd](a))


if __name__ == "__main__":
    main()
