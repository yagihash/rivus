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
  PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> detected=<yes|no|n/a>
with the default flags of rivus. PASS needs all four: it ended by itself within the time limit, the memory of the
process group never went above the cap (a watchdog kills the group of this run only), it converged, and the
count of findings of the category of the target is higher on the pre-fix commit than on the fixed one (counts
only; n/a when the target has no pair). A target that is not decided yet is printed as INFO, not PASS/FAIL.
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


def judge(time_s, limit_s, mem_gb, cap_gb, converged, detected):
    """The pass line, pure. detected: True / False / None (no pair for this target). Returns (passed, line_body)."""
    ok_time = time_s is not None and time_s <= limit_s
    ok_mem = mem_gb is not None and mem_gb <= cap_gb
    ok_det = detected is not False  # None (n/a) does not count against
    passed = bool(ok_time and ok_mem and converged and ok_det)
    body = "time=%s/%s mem=%s/%s converged=%s detected=%s" % (
        "%.0f" % time_s if time_s is not None else "-", "%g" % limit_s,
        "%.2f" % mem_gb if mem_gb is not None else "-", "%g" % cap_gb,
        "yes" if converged else "no", "n/a" if detected is None else ("yes" if detected else "no"))
    return passed, body


def format_line(name, decided, passed, body):
    return "%s %s %s" % (("PASS" if passed else "FAIL") if decided else "INFO", name, body)


def count_category(rep, category):
    return sum(1 for f in rep.get("findings", []) if f.get("category") == category)


def detection(pre_count, fixed_count):
    """Detected when the pre-fix commit has more findings of the category than the fixed one (counts only);
    None for a count that is missing (a run that did not end converged)."""
    if pre_count is None or fixed_count is None:
        return False
    return pre_count > fixed_count


def process_group_rss(pgid):
    """Sum of the RSS (bytes) of the processes of this group; ps only (nothing is signalled)."""
    r = subprocess.run(["ps", "-A", "-o", "pgid=,rss="], capture_output=True, text=True)
    tot = 0
    for l in r.stdout.splitlines():
        f = l.split()
        if len(f) == 2 and f[0] == str(pgid) and f[1].isdigit():
            tot += int(f[1]) * 1024
    return tot


def run_watched(cmd, cwd, env, limit_s, cap_bytes, errpath, outpath):
    """Run cmd in its own session; kill that group (only) when the wall time goes over limit_s or the sum of the RSS
    of the group over cap_bytes. Returns (returncode, why_killed or "", wall_s, peak_bytes)."""
    with open(outpath, "wb") as out, open(errpath, "wb") as err:
        p = subprocess.Popen(cmd, cwd=cwd, env=env, stdout=out, stderr=err, start_new_session=True)
    t0 = time.time()
    peak, why = 0, ""
    pgid = p.pid
    while p.poll() is None:
        peak = max(peak, process_group_rss(pgid))
        if peak > cap_bytes:
            why = "memory"
        elif time.time() - t0 > limit_s:
            why = "time"
        if why:
            if os.getpgid(p.pid) != p.pid or p.pid == os.getpgrp():
                raise SystemExit("deps_bench: refusing to kill process group %d (not the run's own)" % pgid)
            os.killpg(pgid, signal.SIGKILL)
            p.wait()
            break
        time.sleep(0.25)
    return p.returncode, why, time.time() - t0, peak


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
    """One run with the default flags (t["args"]). Returns dict: wall, peak, converged, findings (report or None)."""
    timer = ["/usr/bin/time", "-l"] if sys.platform == "darwin" else ["/usr/bin/time", "-v"]
    cmd = timer + [rivus, "analyze", "-lang", t["lang"], "-C", src] + expand_args(t.get("args", []), src, t.get("subdir", ""))
    outp, errp = os.path.join(work, tag + ".out.json"), os.path.join(work, tag + ".err")
    rc, why, wall, peak = run_watched(cmd, ROOT, env, limit_s, cap_bytes, errp, outp)
    rss = max_rss_mb(open(errp, errors="replace").read())
    if rss is not None:
        peak = max(peak, rss * 1024 * 1024)
    rep, converged = None, False
    if not why and rc == 0:
        try:
            rep = json.load(open(outp))
            st = rep.get("stats", {})
            converged = bool(st.get("converged", False)) and not st.get("timed_out", False)
        except (OSError, ValueError):
            rep = None
    return {"wall": wall, "peak": peak, "converged": converged, "why": why, "report": rep}


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
        detected, d = None, t.get("detect")
        if d:
            pre = run(d["prefix_commit"], "prefix")
            fix = run(d["fixed_commit"], "fixed")
            cnt = lambda r: count_category(r["report"], d["category"]) if r["converged"] else None
            detected = detection(cnt(pre), cnt(fix))
            print("deps_bench: %s %s findings of %s: pre-fix %s, fixed %s" % (name, d["id"], d["category"], cnt(pre), cnt(fix)), file=sys.stderr)
        passed, body = judge(main["wall"], limit_s, main["peak"] / GB, cap_gb, main["converged"], detected)
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
