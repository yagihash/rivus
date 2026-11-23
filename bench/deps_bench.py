#!/usr/bin/env python3
"""The benches with heavy dependencies, run the way a user runs rivus: `rivus analyze` with no flag that
changes the analysis (docs/DESIGN-deps-cost-status.md). One row per bench is appended to bench/deps-ledger.tsv
and printed, so that two dates can be compared. Local only (it is heavy; not in CI): `make bench-deps`.

  bench/deps_bench.py [-timeout 600] [-rivus PATH] [-out bench/deps-ledger.tsv] [bench ...]
  bench/deps_bench.py [-limit S] [-cap GB] [-passline-ledger FILE] memos dart_services   (the pass line, below)

The benches: go-s3 (bench/go-s3), gcp (bench/gcp), dart-gcp (bench/dart-gcp, run in a copy: `dart pub get
--offline`; the packages are read, never run). There is no Rust bench with its dependencies ready, so none is run.

The run is stopped in two ways: `-timeout` of rivus (it reports what it found; stats.timed_out), and, if rivus
does not stop 120 s after that, a kill of the process group of this run only (it is started in its own session;
nothing outside it is signalled). `finished`: yes (stats.timed_out is false and the run converged), timeout,
oom (the process was killed by the system or said it was out of memory), error (anything else).
Columns: date, rivus commit, bench, lang, wall_s, max_rss_mb (/usr/bin/time -l on macOS, -v on Linux; "-" when
the run was killed by us), functions (stats.functions), findings, uncertainties, finished, timeout_s.

The pass line (`make bench-passline`; targets in bench/deps-targets.json): one line per target,
  PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> detected=<yes|no|n/a> quiet_after_fix=<yes|no|n/a>
with the default flags of rivus. PASS needs all four: it ended by itself within the time limit, the memory of the
child ended at or below the cap (b: the maximum over fine samples of the SAME-TIME SUM of the RSS of the whole process tree, the burden the machine receives; a, the end-of-run peak of the main process, is printed beside it; the watchdog stops a run early and kills the group of this run only), it converged (with no function
stopped on the clock: stats.load_dependent is not set), and the
public, already-fixed vulnerability is detected at the PRE-FIX commit (`detected`: at least one finding of the
category of the target; builtin models only; counts only; n/a when the target has no pair).
`quiet_after_fix` (no finding of that category at the fixed commit; read only when detected, n/a otherwise) is
printed beside it and is NOT part of PASS/FAIL: a fix of the kind "validate, then return early" is not treated as
removing the flow until the guard mechanism (#169) exists. A target with `detect.mode` "through" (a truth file, one public fixed
vulnerability) counts per vulnerability: detected when at least one finding passes through a labelled position;
`through_tp=<n>/<N>` (the labelled positions passed through) is printed beside it as information. A target that is not decided yet is printed as INFO, not PASS/FAIL.
Sources are fetched into $BENCH_CACHE (default ~/.cache/rivus-bench/), outside this tree; the output of the runs
goes to $BENCH_OUT (default .claude/work/bench-out/, git-ignored). Nothing of a target is run.
"""
import argparse
import datetime
import json
import os
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from heavylock import heavy_lock  # one lock for the heavy runs of the whole machine (bench/heavylock.py)

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BENCHES = {
    "go-s3": ("bench/go-s3", "go"),
    "gcp": ("bench/gcp", "go"),
    "dart-gcp": ("bench/dart-gcp", "dart"),
}
COLUMNS = ["date", "rivus", "bench", "lang", "wall_s", "max_rss_mb", "functions", "findings", "uncertainties", "finished", "timeout_s"]
GRACE = 120


def run_group(cmd, cwd, env, limit, errpath, outpath):
    """Run cmd in its own session; kill that group (only) after limit seconds. Returns (returncode, killed)."""
    with open(outpath, "wb") as out, open(errpath, "wb") as err:
        p = subprocess.Popen(cmd, cwd=cwd, env=env, stdout=out, stderr=err, start_new_session=True)
    killed = False
    try:
        p.wait(timeout=limit)
    except subprocess.TimeoutExpired:
        pgid = os.getpgid(p.pid)
        if pgid == os.getpgrp() or pgid != p.pid:
            raise SystemExit("deps_bench: refusing to kill process group %d (not the run's own)" % pgid)
        os.killpg(pgid, signal.SIGKILL)
        p.wait()
        killed = True
    return p.returncode, killed


def max_rss_mb(err):
    m = re.search(r"^\s*(\d+)\s+maximum resident set size", err, re.M)  # macOS: bytes
    if m:
        return int(m.group(1)) // (1024 * 1024)
    m = re.search(r"Maximum resident set size \(kbytes\): (\d+)", err)  # Linux: KB
    if m:
        return int(m.group(1)) // 1024
    return None


def max_rss_bytes(err):
    """The end-of-run peak (bytes) from the output of /usr/bin/time (macOS -l: bytes; Linux -v: KB), or None."""
    m = re.search(r"^\s*(\d+)\s+maximum resident set size", err, re.M)
    if m:
        return int(m.group(1))
    m = re.search(r"Maximum resident set size \(kbytes\): (\d+)", err)
    if m:
        return int(m.group(1)) * 1024
    return None


max_rss_mb_exact = max_rss_bytes


def measure(name, rivus, timeout, work):
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
    timer = ["/usr/bin/time", "-l"] if sys.platform == "darwin" else ["/usr/bin/time", "-v"]
    cmd = timer + [rivus, "analyze", "-lang", lang, "-C", src, "-timeout", "%ds" % timeout]
    outp, errp = os.path.join(work, name + ".out.json"), os.path.join(work, name + ".err")
    with heavy_lock("deps_bench " + name):
        t0 = time.time()
        rc, killed = run_group(cmd, ROOT, env, timeout + GRACE, errp, outp)
    wall = time.time() - t0
    err = open(errp, errors="replace").read()
    row = {"bench": name, "lang": lang, "wall_s": "%.1f" % wall, "timeout_s": timeout, "functions": "-", "findings": "-", "uncertainties": "-"}
    rss = max_rss_mb(err)
    row["max_rss_mb"] = rss if rss is not None else "-"
    try:
        rep = json.load(open(outp))
        st = rep.get("stats", {})
        row.update(functions=st.get("functions", "-"), findings=len(rep.get("findings", [])), uncertainties=len(rep.get("uncertainties", [])))
        if st.get("timed_out") or killed:
            row["finished"] = "timeout"
        else:
            row["finished"] = "yes" if st.get("converged", True) else "error"
    except (OSError, ValueError):
        if killed:
            row["finished"] = "timeout"
        elif rc in (-9, 137) or re.search(r"out of memory|cannot allocate memory", err, re.I):
            row["finished"] = "oom"
        else:
            row["finished"] = "error"
    return row, None



TARGETS_FILE = os.path.join(ROOT, "bench/deps-targets.json")
PASSLINE_LEDGER = os.path.join(ROOT, "bench/passline-ledger.tsv")
PASSLINE_COLUMNS = ["date", "rivus", "line"]
GB = 1024 ** 3


def judge(time_s, limit_s, mem_gb, cap_gb, converged, detected, through=None, quiet_after_fix=None):
    """The pass line, pure. detected: True / False / None (no pair for this target). through: None or (tp, n), the
    position-level count, printed as information only (it does not decide PASS). quiet_after_fix: True / False /
    None (n/a); printed, and NOT part of PASS/FAIL. Returns (passed, line_body)."""
    ok_time = time_s is not None and time_s <= limit_s
    ok_mem = mem_gb is not None and mem_gb <= cap_gb
    ok_det = detected is not False  # None (n/a) does not count against
    passed = bool(ok_time and ok_mem and converged and ok_det)
    yn = lambda v: "n/a" if v is None else ("yes" if v else "no")
    body = "time=%s/%s mem=%s/%s converged=%s detected=%s quiet_after_fix=%s" % (
        "%.0f" % time_s if time_s is not None else "-", "%g" % limit_s,
        "%.2f" % mem_gb if mem_gb is not None else "-", "%g" % cap_gb,
        "yes" if converged else "no", yn(detected), yn(quiet_after_fix))
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
    src = os.path.join(copy, t.get("subdir", ""))
    r = subprocess.run(["dart", "pub", "get"], cwd=src, capture_output=True, text=True)
    if r.returncode != 0:
        return None, env, "dart pub get failed: " + r.stderr.strip()[-200:]
    env = dict(env)
    env["RIVUS_DART"] = "dart"
    env["RIVUS_DART_EXTRACTOR"] = "dart run " + os.path.join(ROOT, "extractor/dart/bin/rivus_dart.dart")
    return src, env, None


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
        m = run_watched(cmd, ROOT, env, limit_s, cap_bytes, errp, outp)
    rc, why, wall, peak = m["rc"], m["why"], m["wall"], m["b"]
    rep, converged = None, False
    if not why and rc == 0:
        try:
            rep = json.load(open(outp))
            st = rep.get("stats", {})
            # a run that stopped a function on the clock (-func-timeout, -timeout) depends on the load of the machine:
            # the same run on a quieter one may find more, so it does not count as converged (stats.load_dependent)
            converged = bool(st.get("converged", False)) and not st.get("timed_out", False) and not st.get("load_dependent", False)
        except (OSError, ValueError):
            rep = None
    return {"wall": wall, "peak": peak, "converged": converged, "why": why, "report": rep, "peak_how": "tree-sum", "peak_a": m["a"], "cpu": m["cpu"], "big": m["big"], "big_rss": m["big_rss"], "at_s": m["at_s"]}


def passline(name, t, rivus, cache, outdir, limit_s, cap_gb):
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

        main = run(t.get("commit", ""), "pin")
        detected, quiet, d, through = None, None, t.get("detect"), None
        if d and d.get("mode") == "through":  # per vulnerability; the position count beside it (information)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            tp, n = through_counts(truth, main["report"]) if main["converged"] else (None, len([x for x in truth if "file" in x and x.get("label") == "vuln"]))
            detected, through = detected_through(tp), (tp, n)
        elif d:
            cnt = lambda r: count_category(r["report"], d["category"]) if r["converged"] else None
            pre = run(d["prefix_commit"], "prefix")
            detected = detection(cnt(pre))
            fixed_n = None
            if detected:  # the fixed commit is read only to record quiet_after_fix (not part of PASS/FAIL)
                fixed_n = cnt(run(d["fixed_commit"], "fixed"))
                quiet = quiet_after_fix(fixed_n)
            print("deps_bench: %s %s findings of %s: pre-fix %s, fixed %s" % (name, d["id"], d["category"], cnt(pre), fixed_n), file=sys.stderr)
        passed, body = judge(main["wall"], limit_s, main["peak"] / GB, cap_gb, main["converged"], detected, through, quiet)
        return format_line(name, t.get("decided", False), passed, body)


def run_passline(names, a):
    targets = json.load(open(TARGETS_FILE))
    cache, outdir = cache_dir(), out_dir()
    commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as bw:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(bw, "rivus")
            subprocess.run(["go", "build", "-o", rivus, "./cmd/rivus"], cwd=ROOT, check=True)
        for name in names:
            t = targets[name]
            line = passline(name, t, rivus, cache, outdir, a.limit or t["time_limit_s"], a.cap or t["mem_cap_gb"])
            print(line, flush=True)
            if a.passline_ledger:
                new = not os.path.exists(a.passline_ledger)
                with open(a.passline_ledger, "a") as f:
                    if new:
                        f.write("\t".join(PASSLINE_COLUMNS) + "\n")
                    f.write("\t".join([datetime.date.today().isoformat(), commit, line]) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-timeout", type=int, default=600, help="seconds (the -timeout of rivus; the group is killed GRACE s later)")
    ap.add_argument("-rivus", default="", help="the rivus binary (default: built from this tree)")
    ap.add_argument("-out", default=os.path.join(ROOT, "bench/deps-ledger.tsv"))
    ap.add_argument("-limit", type=float, default=0, help="pass line: the time limit in seconds (default: the one of the target)")
    ap.add_argument("-cap", type=float, default=0, help="pass line: the memory cap in GB (default: the one of the target)")
    ap.add_argument("-passline-ledger", default="", help="append the pass line (date, rivus, line) to this file (bench/passline-ledger.tsv)")
    ap.add_argument("benches", nargs="*", default=list(BENCHES))
    a = ap.parse_args()
    targets = json.load(open(TARGETS_FILE))
    if a.benches and all(b in targets for b in a.benches):
        return run_passline(a.benches, a)
    if any(b in targets for b in a.benches):
        raise SystemExit("deps_bench: do not mix the benches of the ledger and the targets of the pass line")
    commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as work:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(work, "rivus")
            subprocess.run(["go", "build", "-o", rivus, "./cmd/rivus"], cwd=ROOT, check=True)
        new = not os.path.exists(a.out)
        with open(a.out, "a") as f:
            if new:
                f.write("\t".join(COLUMNS) + "\n")
            for name in a.benches:
                row, why = measure(name, rivus, a.timeout, work)
                if row is None:
                    print("deps_bench: %s skipped: %s" % (name, why), file=sys.stderr)
                    continue
                row.update(date=datetime.date.today().isoformat(), rivus=commit)
                line = "\t".join(str(row[c]) for c in COLUMNS)
                f.write(line + "\n")
                f.flush()
                print(line, flush=True)


if __name__ == "__main__":
    main()
