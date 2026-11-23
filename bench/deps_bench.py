#!/usr/bin/env python3
"""The benches with heavy dependencies, run the way a user runs rivus: `rivus analyze` with no flag that
changes the analysis (docs/DESIGN-deps-cost-status.md). One row per bench is appended to bench/deps-ledger.tsv
and printed, so that two dates can be compared. Local only (it is heavy; not in CI): `make bench-deps`.

  bench/deps_bench.py [-timeout 600] [-rivus PATH] [-out bench/deps-ledger.tsv] [bench ...]

The benches: go-s3 (bench/go-s3), gcp (bench/gcp), dart-gcp (bench/dart-gcp, run in a copy: `dart pub get
--offline`; the packages are read, never run). There is no Rust bench with its dependencies ready, so none is run.

The run is stopped in two ways: `-timeout` of rivus (it reports what it found; stats.timed_out), and, if rivus
does not stop 120 s after that, a kill of the process group of this run only (it is started in its own session;
nothing outside it is signalled). `finished`: yes (stats.timed_out is false and the run converged), timeout,
oom (the process was killed by the system or said it was out of memory), error (anything else).
Columns: date, rivus commit, bench, lang, wall_s, max_rss_mb (/usr/bin/time -l on macOS, -v on Linux; "-" when
the run was killed by us), functions (stats.functions), findings, uncertainties, finished, timeout_s.
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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-timeout", type=int, default=600, help="seconds (the -timeout of rivus; the group is killed GRACE s later)")
    ap.add_argument("-rivus", default="", help="the rivus binary (default: built from this tree)")
    ap.add_argument("-out", default=os.path.join(ROOT, "bench/deps-ledger.tsv"))
    ap.add_argument("benches", nargs="*", default=list(BENCHES))
    a = ap.parse_args()
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
