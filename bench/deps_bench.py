#!/usr/bin/env python3
"""The benches with heavy dependencies, run the way a user runs rivus: `rivus analyze` with no flag that
changes the analysis (docs/ROADMAP.md section 3). Every run adds ONE row to the one ledger of the heavy
measurements, bench/heavy-ledger.tsv (bench/heavy_ledger.py: the columns, the writer, the table), unless -no-ledger
(make: NO_LEDGER=1; for a binary with a measurement patch, or a run that is not used as a number). Local only (it is
heavy; not in CI): `make bench-deps`.

  bench/deps_bench.py [-timeout 600] [-rivus PATH] [-ledger FILE] [-no-ledger] [bench ...]
  bench/deps_bench.py [-limit S] [-cap GB] [-ledger FILE] [-no-ledger] memos dart_services   (the pass line, below; dart_services is pass line D: 600 s, 8 GB, converge; takes 10 minutes, select it with TARGETS)

The benches: go-s3 (bench/go-s3), gcp (bench/gcp), dart-gcp (bench/dart-gcp, run in a copy: `dart pub get
--offline`; the packages are read, never run). There is no Rust bench with its dependencies ready, so none is run.

The dependency benches (kind `deps`) run under the same watchdog as the pass line (the sampled memory of the whole
process tree; the cap is 20 GB, only to protect the machine, not a line), with `-timeout` of rivus (it reports what it
found; stats.timed_out) and, if rivus does not stop 120 s after that, a kill of the process group of this run only
(it is started in its own session; nothing outside it is signalled). `result`: `finished` (the run converged and was
not timed out), `timeout` (stats.timed_out, or the group was killed on the clock), `stopped_memory` (the watchdog),
`oom` (killed by the system or out of memory), `error`.

The pass line (`make bench-passline`; targets in bench/deps-targets.json): one line per target,
  PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> finish=<yes|no> detected=<yes|no|n/a> quiet_after_fix=<yes|no|n/a> [located=<n>]
with the default flags of rivus. PASS needs two separate conditions: FINISH (`finish`: it ended by itself within the time limit, the
memory of the child ended at or below the cap (b: the maximum over fine samples of the SAME-TIME SUM of the RSS of the whole process
tree, the burden the machine receives; a, the end-of-run peak of the main process, is printed beside it; the watchdog stops a run
early and kills the group of this run only), it converged, with no function stopped on the clock: stats.load_dependent is not set)
and DETECT (`detected`, by place: at the PRE-FIX commit at least one finding of the category has its source and its sink at the
labelled places of the truth file, bench/memos/truth-*.json, `detect.mode` "location"; builtin models; `located=<n>` is the count of
such findings; n/a when the target has no pair).
`quiet_after_fix` (no finding of that category at the fixed commit; read only when detected, n/a otherwise) is
printed beside it and is NOT part of PASS/FAIL: a fix of the kind "validate, then return early" is not treated as
removing the flow until the guard mechanism (#169) exists. A target with `detect.mode` "through" (a truth file, one public fixed
vulnerability) counts per vulnerability: detected when at least one finding passes through a labelled position;
`through_tp=<n>/<N>` (the labelled positions passed through) is printed beside it as information. A target with `detect.mode` "score" (Gitea) counts per published vulnerability of its truth file (the form of bench/score.py: source
function, sink position, functions on the path; the field `vuln` groups the entries) and prints `detected=<n>/<N>` only: the count is not
part of PASS/FAIL (reference only), and neither the line nor the ledger row carries a count of findings, a list, a path or a place.
A target that is not decided yet is printed as INFO, not PASS/FAIL.
The time limit is kept by the watchdog (a kill of the group of this run at the limit: `stopped_time`); `rivus analyze` is not given `-timeout`
(its default is 0, no limit; a rivus that stopped itself at the limit would look like a run that did not converge), unless the `args` of the target say so.
Sources are fetched into $BENCH_CACHE (default ~/.cache/rivus-bench/), outside this tree; the output of the runs
goes to $BENCH_OUT (default .claude/work/bench-out/, git-ignored; the ledger holds numbers only, the JSON stays there).
Nothing of a target is run.
"""
import argparse
import datetime
import json
import os
import re
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from heavylock import heavy_lock, snapshot, format_snapshots  # one lock for the heavy runs of the whole machine (bench/heavylock.py)
import heavy_ledger  # the one ledger of the heavy measurements (bench/heavy_ledger.py)

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BENCHES = {
    "go-s3": ("bench/go-s3", "go"),
    "gcp": ("bench/gcp", "go"),
    "dart-gcp": ("bench/dart-gcp", "dart"),
}
DEPS_CAP_GB = 20  # the watchdog of the dependency benches: protects the machine; it is not a line
GRACE = 120


def measure(name, rivus, timeout, work):
    """One dependency bench. Returns (row of the ledger without date and commit, None), or (None, why it was skipped)."""
    d, lang = BENCHES[name]
    src = os.path.join(ROOT, d)
    env = dict(os.environ)
    if lang == "dart":
        copy = os.path.join(work, name)
        shutil.copytree(src, copy)
        r = subprocess.run(["dart", "pub", "get", "--offline"], cwd=copy, capture_output=True, text=True)
        if r.returncode != 0:
            return None, "dart pub get --offline failed: " + r.stderr.strip()[-200:]
        src = copy
        env["RIVUS_DART"] = "dart"
        env["RIVUS_DART_EXTRACTOR"] = "dart run " + os.path.join(ROOT, "extractor/dart/bin/rivus_dart.dart")
    cmd = [rivus, "analyze", "-lang", lang, "-C", src, "-timeout", "%ds" % timeout]
    outp, errp = os.path.join(work, name + ".out.json"), os.path.join(work, name + ".err")
    with heavy_lock("deps_bench " + name):
        load = os.getloadavg()[0]
        sn0 = snapshot()
        m = run_watched(cmd, ROOT, env, timeout + GRACE, int(DEPS_CAP_GB * GB), errp, outp)
        note = format_snapshots(sn0, snapshot())
    err = open(errp, errors="replace").read()
    rep = None
    if not m["why"]:
        try:
            rep = json.load(open(outp))
        except (OSError, ValueError):
            rep = None
    st = rep.get("stats", {}) if isinstance(rep, dict) else {}
    converged = bool(st.get("converged", True)) if rep is not None else False
    row = {"target": name, "target_commit": "in-tree", "kind": "deps", "settings": "default (-timeout %ds); %s" % (timeout, note),
           "wall_s": "%.1f" % m["wall"], "cpu_s": "%.1f" % m["cpu"], "peak_tree_gb": "%.2f" % (m["b"] / GB),
           "converged": converged if rep is not None else None, "exit_code": m["rc"], "load_start": "%.2f" % load}
    row.update({k: v for k, v in heavy_ledger.report_columns(rep).items() if v is not None})
    if m["why"] == "memory":
        row["result"] = "stopped_memory"
    elif m["why"] == "time" or st.get("timed_out"):
        row["result"] = "timeout"
    elif rep is not None:
        row["result"] = "finished" if converged else "error"
    elif m["rc"] in (-9, 137) or re.search(r"out of memory|cannot allocate memory", err, re.I):
        row["result"] = "oom"
    else:
        row["result"] = "error"
    return row, None


TARGETS_FILE = os.path.join(ROOT, "bench/deps-targets.json")
GB = 1024 ** 3
EXIT_INCOMPLETE = 4  # rivus: the output is complete as far as it goes, the analysis is not (cmd/rivus exitIncomplete)


def judge(time_s, limit_s, mem_gb, cap_gb, converged, detected, through=None, quiet_after_fix=None, located=None, score=None, coarsened=None, expect_coarsened=0):
    """The pass line, pure: two separate conditions, both needed. FINISH (time within the limit, memory within the cap,
    converged) and DETECT (`detected`). located: None or the count of findings at the labelled places (information). detected: True / False / None (no pair for this target). through: None or (tp, n), the
    position-level count, printed as information only (it does not decide PASS). quiet_after_fix: True / False /
    None (n/a); printed, and NOT part of PASS/FAIL. score: None or (n, N), the vulnerabilities detected by place (detect.mode
    score), printed as `detected=<n>/<N>` only, never part of PASS/FAIL. coarsened: None or stats.scc_coarsened (the components
    whose dependency functions are solved at depth 0: -scc-coarsen, ADR-124); it must be expect_coarsened (0 for a target that is not meant
    to be hit), or the line fails: a target that comes to be hit changes its answer without being said. Returns (passed, line_body)."""
    ok_time = time_s is not None and time_s <= limit_s
    ok_mem = mem_gb is not None and mem_gb <= cap_gb
    if score is not None:
        detected = None  # the count is printed (below) but is not part of the verdict
    ok_det = detected is not False  # None (n/a) does not count against
    finish = bool(ok_time and ok_mem and converged)
    ok_coarse = coarsened is None or coarsened == expect_coarsened
    passed = bool(finish and ok_det and ok_coarse)
    yn = lambda v: "n/a" if v is None else ("yes" if v else "no")
    body = "time=%s/%s mem=%s/%s converged=%s finish=%s detected=%s quiet_after_fix=%s" % (
        "%.0f" % time_s if time_s is not None else "-", "%g" % limit_s,
        "%.2f" % mem_gb if mem_gb is not None else "-", "%g" % cap_gb,
        "yes" if converged else "no", yn(finish), yn(detected) if score is None else "%s/%d" % ("-" if score[0] is None else score[0], score[1]), yn(quiet_after_fix))
    if coarsened is not None:
        body += " coarsened=%d/%d" % (coarsened, expect_coarsened)
    if located is not None:
        body += " located=%d" % located
    if through is not None:
        body += " through_tp=%s/%d" % ("-" if through[0] is None else through[0], through[1])
    return passed, body


def format_line(name, decided, passed, body):
    return "%s %s %s" % (("PASS" if passed else "FAIL") if decided else "INFO", name, body)


def count_category(rep, category):
    return sum(1 for f in rep.get("findings", []) if f.get("category") == category)


def detection(pre_count):
    """Detected: the pre-fix commit has at least one finding of the category (None: the run did not converge)."""
    return pre_count is not None and pre_count > 0


def quiet_after_fix(fixed_count):
    """The fixed commit has no finding of the category (None: the run did not converge, so not quiet)."""
    return fixed_count is not None and fixed_count == 0


def location_counts(truth, rep, category, side="prefix"):
    """Detection by place (detect.mode location; the truth is the list `sites` of bench/memos/truth-*.json: role source /
    sink, file, line = the line at the pre-fix commit, fixed_line = the line at the fixed commit). A finding counts when it is of
    the category AND its source is at a source site AND its sink is at a sink site (file, line of the position of the
    finding). Returns (findings at the places, source sites, sink sites). Counts only; the finding itself is not kept."""
    key = "line" if side == "prefix" else "fixed_line"
    at = lambda role: {(x["file"], x[key]) for x in truth if x.get("role") == role and x.get("category") == category}
    src, snk = at("source"), at("sink")
    pos = lambda e: ((e or {}).get("position") or {})
    n = 0
    for f in (rep or {}).get("findings", []):
        if f.get("category") != category:
            continue
        a, b = pos(f.get("source")), pos(f.get("sink"))
        if (a.get("file"), a.get("line")) in src and (b.get("file"), b.get("line")) in snk:
            n += 1
    return n, len(src), len(snk)


def detected_located(n):
    """Per vulnerability: detected when at least one finding is at the labelled places (None: the run did not converge)."""
    return n is not None and n >= 1


def through_counts(truth, rep):
    """Position level (the same scoring as `bench/score.py --through`): a labelled "vuln" site (file, line) counts when a
    finding of its category has a path with a `call ...` step at that position (kinds that differ do not count).
    Returns (tp, n). Counts only."""
    call_steps = {}
    for f in (rep or {}).get("findings", []):
        for pa in f.get("paths") or []:
            for st in pa.get("steps", []):
                if st.get("desc", "").startswith("call "):
                    q = st["position"]
                    call_steps.setdefault((q["file"], q["line"]), set()).add(f["category"])
    sites = [t for t in truth if "file" in t and t.get("label") == "vuln"]
    tp = sum(1 for t in sites if t["category"] in call_steps.get((t["file"], t["line"]), ()))
    return tp, len(sites)


def detected_through(tp):
    """Per vulnerability: detected when at least one finding passes through a labelled position of its truth."""
    return tp is not None and tp >= 1


def score_counts(truth, rep):
    """Detection by place for detect.mode score (the form of bench/score.py: source function, sink file and line, `via`: the
    functions on one path). The truth is the list `sites` of bench/gitea-cve-truth.json; the field `vuln` groups the entries of
    one published vulnerability, which counts as detected when one of its "vuln" entries is hit. Returns (detected, N).
    Counts only: nothing of a finding is kept or returned."""
    pair = {}  # (source func, sink file, sink line, category) -> [set of the functions of each path, with the callees]
    for f in (rep or {}).get("findings", []):
        p = (f.get("sink") or {}).get("position") or {}
        fs_all = pair.setdefault(((f.get("source") or {}).get("func", ""), p.get("file"), p.get("line"), f.get("category")), [])
        for pa in f.get("paths") or []:
            fs = set()
            for st in pa.get("steps", []):
                fs.add(st.get("func", ""))
                dsc = st.get("desc", "")
                if dsc.startswith("call "):
                    fs.add(dsc[5:])
            fs_all.append(fs)

    def hit(s):
        for (fn, sf, sl, cat), paths in pair.items():
            if not (fn.endswith(s["source_func"]) and (sf, sl) == (s["sink_file"], s["sink_line"]) and cat == s["category"]):
                continue
            via = s.get("via") or []
            if not via or any(all(any(x.endswith(v) for x in fs) for v in via) for fs in paths):
                return True
        return False

    groups = {}
    for s in truth:
        if s.get("label") == "vuln":
            groups[s["vuln"]] = groups.get(s["vuln"], False) or hit(s)
    return sum(groups.values()), len(groups)


def tree_rows(root_pid, pgid):
    """The processes of the tree of the run: the process, its descendants (by parent), and the members of its group.
    Returns [(pid, rss_bytes, comm)]; ps only (nothing is signalled)."""
    r = subprocess.run(["ps", "-A", "-o", "pid=,ppid=,pgid=,rss=,comm="], capture_output=True, text=True)
    rows = {}
    kids = {}
    for l in r.stdout.splitlines():
        f = l.split(None, 4)
        if len(f) == 5 and f[0].isdigit() and f[1].isdigit() and f[2].isdigit() and f[3].isdigit():
            pid, ppid, pg = int(f[0]), int(f[1]), int(f[2])
            rows[pid] = (pg, int(f[3]) * 1024, os.path.basename(f[4]))
            kids.setdefault(ppid, []).append(pid)
    seen, todo = set(), [root_pid]
    while todo:
        x = todo.pop()
        if x in seen or x not in rows:
            continue
        seen.add(x)
        todo.extend(kids.get(x, []))
    seen |= {pid for pid, v in rows.items() if v[0] == pgid}
    return [(pid, rows[pid][1], rows[pid][2]) for pid in sorted(seen)]


def maxrss_bytes(ru):
    """ru_maxrss of a wait4 result: bytes on macOS, KB on Linux."""
    return int(ru.ru_maxrss) * (1 if sys.platform == "darwin" else 1024)


def run_watched(cmd, cwd, env, limit_s, cap_bytes, errpath, outpath, interval=0.25):
    """Run cmd in its own session. Two records of memory, and a WATCHDOG that is not a record:
      a = the end-of-run peak of the main process (ru_maxrss of wait4: the largest single process of its subtree);
      b = the maximum over fine samples (every `interval` s) of the SAME-TIME SUM of the RSS of the whole process tree
          (shared pages can be counted twice); `big` names the biggest process of the tree at that moment, and `at_s` when.
    The watchdog kills the group (only) when the wall time is over limit_s or the sum b is over cap_bytes.
    Returns a dict: rc, why ("" / "time" / "memory"), wall, cpu, a, b, big, big_rss, at_s."""
    with open(outpath, "wb") as out, open(errpath, "wb") as err:
        p = subprocess.Popen(cmd, cwd=cwd, env=env, stdout=out, stderr=err, start_new_session=True)
    t0 = time.time()
    pgid = p.pid
    b, big, big_rss, at_s, why = 0, "", 0, 0.0, ""
    rc, ru = None, None
    while True:
        wp, status, ru_ = os.wait4(p.pid, os.WNOHANG)
        if wp:
            rc, ru = os.waitstatus_to_exitcode(status), ru_
            break
        rows = tree_rows(p.pid, pgid)
        tot = sum(r[1] for r in rows)
        if tot > b:
            top = max(rows, key=lambda r: r[1])
            b, big, big_rss, at_s = tot, top[2], top[1], time.time() - t0
        if b > cap_bytes:
            why = "memory"
        elif time.time() - t0 > limit_s:
            why = "time"
        if why:
            if os.getpgid(p.pid) != p.pid or p.pid == os.getpgrp():
                raise SystemExit("deps_bench: refusing to kill process group %d (not the run's own)" % pgid)
            os.killpg(pgid, signal.SIGKILL)
            _, status, ru = os.wait4(p.pid, 0)
            rc = os.waitstatus_to_exitcode(status)
            break
        time.sleep(interval)
    return {"rc": rc, "why": why, "wall": time.time() - t0, "cpu": ru.ru_utime + ru.ru_stime,
            "a": maxrss_bytes(ru), "b": b, "big": big, "big_rss": big_rss, "at_s": at_s}


def git(*args, cwd=None):
    # no hooks, no filters that run programs; fetch and checkout only
    subprocess.run(["git", "-c", "core.hooksPath=/dev/null"] + list(args), cwd=cwd, check=True, capture_output=True)


def cache_dir():
    d = os.environ.get("BENCH_CACHE") or os.path.expanduser("~/.cache/rivus-bench")
    d = os.path.abspath(d)
    if d == ROOT or d.startswith(ROOT + os.sep):
        raise SystemExit("deps_bench: BENCH_CACHE must be outside the repository (%s)" % d)
    os.makedirs(d, exist_ok=True)
    return d


def out_dir():
    d = os.path.abspath(os.environ.get("BENCH_OUT") or os.path.join(ROOT, ".claude/work/bench-out"))
    if d == ROOT or d.startswith(ROOT + os.sep):
        r = subprocess.run(["git", "check-ignore", "-q", os.path.join(d, "x")], cwd=ROOT)
        if r.returncode != 0:
            raise SystemExit("deps_bench: BENCH_OUT %s is inside the repository and not git-ignored" % d)
    os.makedirs(d, exist_ok=True)
    return d


def fetch(name, t, commit, cache):
    """The tree of the target at commit, in the cache. Returns its path. Fetch and checkout only."""
    base = os.path.join(cache, name)
    os.makedirs(base, exist_ok=True)
    main = os.path.join(base, "src")
    if not os.path.isdir(os.path.join(main, ".git")):
        git("clone", "--no-checkout", t["repo"], main)
    short = commit[:12]
    wt = os.path.join(base, "wt-" + short)
    if not os.path.isdir(wt):
        try:
            git("cat-file", "-e", commit + "^{commit}", cwd=main)
        except subprocess.CalledProcessError:
            git("fetch", "origin", commit, cwd=main)
        git("worktree", "add", "--detach", wt, commit, cwd=main)
    return wt


def prepare(name, t, tree, work, env):
    """Returns (analysis dir, env, error)."""
    src = os.path.join(tree, t.get("subdir", ""))
    if t["lang"] == "go":
        r = subprocess.run(["go", "mod", "download"], cwd=src, capture_output=True, text=True)
        if r.returncode != 0:
            return None, env, "go mod download failed: " + r.stderr.strip()[-200:]
        return src, env, None
    copy = os.path.join(work, os.path.basename(tree))
    if not os.path.isdir(copy):
        shutil.copytree(tree, copy, ignore=shutil.ignore_patterns(".git"))
    if t.get("dart_remove_flutter_workspace_members"):
        remove_flutter_members(copy)
    pin_lock(t, copy)
    src = os.path.join(copy, t.get("subdir", ""))
    r = subprocess.run(["dart", "pub", "get"], cwd=src, capture_output=True, text=True)
    if r.returncode != 0:
        return None, env, "dart pub get failed: " + r.stderr.strip()[-200:]
    env = dict(env)
    env["RIVUS_DART"] = "dart"
    env["RIVUS_DART_EXTRACTOR"] = "dart run " + os.path.join(ROOT, "extractor/dart/bin/rivus_dart.dart")
    return src, env, None


def pin_lock(t, root):
    """In the COPY: put the pinned `pubspec.lock` of the target (the `lock` field, a path in this repository) at the root of the tree, so `dart pub get` keeps those versions. No `lock` field: nothing is written."""
    lock = t.get("lock")
    if not lock:
        return
    shutil.copyfile(os.path.join(ROOT, lock), os.path.join(root, "pubspec.lock"))


def remove_flutter_members(root):
    """In the COPY: drop from the root pubspec the `workspace:` members that need Flutter."""
    path = os.path.join(root, "pubspec.yaml")
    lines = open(path).read().split("\n")
    out, in_ws = [], False
    for l in lines:
        if re.match(r"^workspace:\s*$", l):
            in_ws = True
            out.append(l)
            continue
        if in_ws:
            m = re.match(r"^\s*-\s*(\S+)\s*$", l)
            if m:
                pub = os.path.join(root, m.group(1), "pubspec.yaml")
                if os.path.exists(pub) and re.search(r"^\s*sdk:\s*flutter\s*$|^\s*flutter:\s*$", open(pub).read(), re.M):
                    continue
            elif l.strip():
                in_ws = False
        out.append(l)
    open(path, "w").write("\n".join(out))


def expand_args(args, src, subdir):
    """The flags of the target; `{root}` is the root of the tree (src without its subdir), e.g. for -allow-deps-under."""
    root = os.path.normpath(os.path.join(src, *([".."] * len([x for x in subdir.split("/") if x])))) if subdir else src
    return [a.replace("{root}", root) for a in args]


def analyze_once(rivus, t, src, env, limit_s, cap_bytes, work, tag):
    """One run with the default flags (t["args"]). Returns dict: wall, cpu, peak (= b, the whole process tree: the
    memory of the verdict), peak_a (the main process), peak_how, big, converged, why, report."""
    cmd = [rivus, "analyze", "-lang", t["lang"], "-C", src] + expand_args(t.get("args", []), src, t.get("subdir", ""))
    outp, errp = os.path.join(work, tag + ".out.json"), os.path.join(work, tag + ".err")
    with heavy_lock("deps_bench analyze " + tag):
        load = os.getloadavg()[0]
        sn0 = snapshot()
        m = run_watched(cmd, ROOT, env, limit_s, cap_bytes, errp, outp)
        note = format_snapshots(sn0, snapshot())
    rc, why, wall, peak = m["rc"], m["why"], m["wall"], m["b"]
    rep, converged = None, False
    if not why and rc in (0, EXIT_INCOMPLETE):  # 4: the report is written, but the analysis is not complete (timeout, not converged); converged below says it
        try:
            rep = json.load(open(outp))
            st = rep.get("stats", {})
            # a run that stopped a function on the clock (-func-timeout, -timeout) depends on the load of the machine:
            # the same run on a quieter one may find more, so it does not count as converged (stats.load_dependent)
            converged = bool(st.get("converged", False)) and not st.get("timed_out", False) and not st.get("load_dependent", False)
        except (OSError, ValueError):
            rep = None
    return {"wall": wall, "peak": peak, "converged": converged, "why": why, "report": rep, "peak_how": "tree-sum", "peak_a": m["a"], "cpu": m["cpu"], "big": m["big"], "big_rss": m["big_rss"], "at_s": m["at_s"], "rc": rc, "load": load, "note": note}


def median_run(runs):
    """One run for n runs of the same target: the run of the median wall time, with the median wall, cpu and peak; converged only when all did.
    The notes (load, busy processes) of every run are kept."""
    if len(runs) == 1:
        return runs[0]
    by = sorted(runs, key=lambda r: r["wall"])
    r = dict(by[(len(by) - 1) // 2])
    for k in ("wall", "cpu", "peak"):
        r[k] = statistics.median(x[k] for x in runs)
    r["converged"] = all(x["converged"] for x in runs)
    r["why"] = next((x["why"] for x in runs if x["why"]), "")
    r["note"] = " | ".join("run %d: %s" % (i + 1, x["note"]) for i, x in enumerate(runs))
    r["walls"] = [x["wall"] for x in runs]
    return r


def passline(name, t, rivus, cache, outdir, limit_s, cap_gb, repeat=1):
    with tempfile.TemporaryDirectory(prefix="passline-") as work:
        env = dict(os.environ)
        cap = int(cap_gb * GB)

        def tree_for(commit):
            if "local" in t:
                return os.path.join(ROOT, t["local"])
            return fetch(name, t, commit, cache)

        def run(commit, tag):
            tree = tree_for(commit)
            src, e, err = prepare(name, t, tree, work, env)
            if err:
                raise SystemExit("deps_bench: %s: %s" % (name, err))
            r = analyze_once(rivus, t, src, e, limit_s, cap, work, tag)
            if r["report"] is not None:  # the report goes to the untracked dir; counts are read from it
                shutil.copy(os.path.join(work, tag + ".out.json"), os.path.join(outdir, "%s-%s.json" % (name, tag)))
            return r

        main = median_run([run(t.get("commit", ""), "pin" if i == 0 else "pin%d" % (i + 1)) for i in range(max(1, repeat))])
        detected, quiet, d, through, located, score = None, None, t.get("detect"), None, None, None
        if d and d.get("mode") == "score":  # per published vulnerability, by place; the count n/N only (no finding count, list, path or place anywhere)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            total = len({x["vuln"] for x in truth if x.get("label") == "vuln"})
            score = score_counts(truth, main["report"]) if main["converged"] else (None, total)
        elif d and d.get("mode") == "through":  # per vulnerability; the position count beside it (information)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            tp, n = through_counts(truth, main["report"]) if main["converged"] else (None, len([x for x in truth if "file" in x and x.get("label") == "vuln"]))
            detected, through = detected_through(tp), (tp, n)
        elif d and d.get("mode") == "location":  # per vulnerability, by the places of the truth (the pre-fix commit)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            cat = d["category"]
            cnt = lambda r: count_category(r["report"], cat) if r["converged"] else None
            loc = lambda r, side: location_counts(truth, r["report"], cat, side)[0] if r["converged"] else None
            pre = run(d["prefix_commit"], "prefix")
            located = loc(pre, "prefix")
            detected = detected_located(located)
            fixed_n = fixed_loc = None
            if detected:  # the fixed commit is read only to record quiet_after_fix (not part of PASS/FAIL)
                fx = run(d["fixed_commit"], "fixed")
                fixed_n, fixed_loc = cnt(fx), loc(fx, "fixed")
                quiet = quiet_after_fix(fixed_n)
            print("deps_bench: %s %s findings of %s: pre-fix %s (at the labelled places %s), fixed %s (at the places of the fixed commit %s)" % (name, d["id"], cat, cnt(pre), located, fixed_n, fixed_loc), file=sys.stderr)
        elif d:
            cnt = lambda r: count_category(r["report"], d["category"]) if r["converged"] else None
            pre = run(d["prefix_commit"], "prefix")
            detected = detection(cnt(pre))
            fixed_n = None
            if detected:  # the fixed commit is read only to record quiet_after_fix (not part of PASS/FAIL)
                fixed_n = cnt(run(d["fixed_commit"], "fixed"))
                quiet = quiet_after_fix(fixed_n)
            print("deps_bench: %s %s findings of %s: pre-fix %s, fixed %s" % (name, d["id"], d["category"], cnt(pre), fixed_n), file=sys.stderr)
        coarsened = ((main["report"] or {}).get("stats") or {}).get("scc_coarsened", 0) if main["report"] is not None else None
        passed, body = judge(main["wall"], limit_s, main["peak"] / GB, cap_gb, main["converged"], detected, through, quiet, located, score, coarsened, t.get("expect_coarsened", 0))
        decided = t.get("decided", False)
        if main["why"]:
            result = "stopped_" + main["why"]
        else:
            result = ("PASS" if passed else "FAIL") if decided else "INFO"
        extra = [a for a in t.get("args", [])] + (["-limit %g" % limit_s] if limit_s != t["time_limit_s"] else []) + (["-cap %g" % cap_gb] if cap_gb != t["mem_cap_gb"] else [])
        extra += ["%s=%s" % (k, v) for k, v in sorted(env.items()) if k == "GOMEMLIMIT" or (k.startswith("RIVUS_") and k not in ("RIVUS_HEAVY_LOCK_HELD", "RIVUS_HEAVY_LOCK"))]
        if repeat > 1:
            extra.append("repeat %d (median)" % repeat)
        row = {"target": name, "target_commit": t.get("commit") or "in-tree", "kind": "passline", "settings": "%s; %s" % (" ".join(extra) or "default", main["note"]),
               "wall_s": "%.1f" % main["wall"], "cpu_s": "%.1f" % main["cpu"], "peak_tree_gb": "%.2f" % (main["peak"] / GB),
               "converged": main["converged"], "detected": detected if score is None else "%s/%d" % ("-" if score[0] is None else score[0], score[1]), "quiet_after_fix": quiet, "exit_code": main["rc"],
               "load_start": "%.2f" % main["load"], "result": result}
        row.update({k: v for k, v in heavy_ledger.report_columns(main["report"]).items() if v is not None})
        if score is not None:
            row.pop("findings", None)  # the count of findings of this target is not written
        return format_line(name, decided, passed, body), row


def git_commit():
    return subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()


def add_to_ledger(row, a, commit):
    """One row for a run, unless -no-ledger. date and rivus_commit are added here."""
    if a.no_ledger:
        return
    row = dict(row, date=datetime.date.today().isoformat(), rivus_commit=commit)
    heavy_ledger.append_row(row, a.ledger)


def run_passline(names, a):
    targets = json.load(open(TARGETS_FILE))
    cache, outdir = cache_dir(), out_dir()
    commit = git_commit()
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as bw:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(bw, "rivus")
            subprocess.run(["go", "build", "-o", rivus, "./cmd/rivus"], cwd=ROOT, check=True)
        for name in names:
            t = targets[name]
            line, row = passline(name, t, rivus, cache, outdir, a.limit or t["time_limit_s"], a.cap or t["mem_cap_gb"], a.repeat)
            print(line, flush=True)
            add_to_ledger(row, a, commit)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-timeout", type=int, default=600, help="seconds (the -timeout of rivus; the group is killed GRACE s later)")
    ap.add_argument("-rivus", default="", help="the rivus binary (default: built from this tree)")
    ap.add_argument("-ledger", default=heavy_ledger.LEDGER, help="the ledger (bench/heavy-ledger.tsv)")
    ap.add_argument("-no-ledger", action="store_true", help="do not add a row (a binary with a measurement patch; a run that is not used as a number)")
    ap.add_argument("-limit", type=float, default=0, help="pass line: the time limit in seconds (default: the one of the target)")
    ap.add_argument("-cap", type=float, default=0, help="pass line: the memory cap in GB (default: the one of the target)")
    ap.add_argument("-repeat", "--repeat", type=int, default=1, help="pass line: run each target n times; the median of the time, the CPU and the peak goes to the verdict (converged: all runs). The decision of D is the median of 3 on a quiet machine")
    ap.add_argument("benches", nargs="*", default=list(BENCHES))
    a = ap.parse_args()
    targets = json.load(open(TARGETS_FILE))
    if a.benches and all(b in targets for b in a.benches):
        return run_passline(a.benches, a)
    if any(b in targets for b in a.benches):
        raise SystemExit("deps_bench: do not mix the benches of the ledger and the targets of the pass line")
    commit = git_commit()
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as work:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(work, "rivus")
            subprocess.run(["go", "build", "-o", rivus, "./cmd/rivus"], cwd=ROOT, check=True)
        for name in a.benches:
            row, why = measure(name, rivus, a.timeout, work)
            if row is None:
                print("deps_bench: %s skipped: %s" % (name, why), file=sys.stderr)
                continue
            add_to_ledger(row, a, commit)
            print("%s %s wall=%ss tree=%sGB result=%s" % (name, row["kind"], row["wall_s"], row["peak_tree_gb"], row["result"]), flush=True)


if __name__ == "__main__":
    main()
