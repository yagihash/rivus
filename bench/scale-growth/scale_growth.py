#!/usr/bin/env python3
"""How the live memory of each stage grows with the size of the target (bench/scale-growth/README.md).

  python3 bench/scale-growth/scale_growth.py [-scales 1,2,4] [-profile mixed|ops|funcs] [-workers 1] [-rivus BIN]

For each scale it makes a synthetic target (bench/go-scale/gen), runs `rivus analyze -v` under GODEBUG=gctrace=1
(the body of rivus is not changed), reads the live memory of two stages from stderr, and prints the growth
between the scales. An investigation, not a pass/fail: nothing goes to bench/heavy-ledger.tsv.
"""
import argparse
import json
import math
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import deps_bench as d  # run_watched (the watchdog), out_dir
from heavylock import heavy_lock

ROOT = d.ROOT
GB = d.GB if hasattr(d, "GB") else 1 << 30
PHASE = re.compile(r"^rivus: (\S+)\s+\S+\s+\(heap (\d+) MB\)")
GCLINE = re.compile(r"^gc \d+ @.*?(\d+)->(\d+)->(\d+) MB")
STAGES = ("load_mb", "summary_mb")

# base values at scale 1; a profile says how each knob moves with the scale k.
PROFILES = {
    # ops, ballast functions and packages all grow with k (the nearest to a real product: more code and more operations)
    "mixed": lambda k: ["-shapes", "stagefield", "-ops", str(40 * k), "-stages", "4", "-fields", "8", "-funcs", str(2000 * k), "-pkgs", str(20 * k)],
    # only the operations (the strongly connected component and the summaries grow; the ballast stays)
    "ops": lambda k: ["-shapes", "stagefield", "-ops", str(40 * k), "-stages", "4", "-fields", "8"],
    # only the ordinary functions (the load grows; no big component)
    "funcs": lambda k: ["-shapes", "none", "-funcs", str(2000 * k), "-pkgs", str(20 * k)],
}


def parse_stderr(text):
    """Reads the stderr of `rivus analyze -v` with GODEBUG=gctrace=1.

    The phase lines (-v) are printed after a forced collection, so their heap is the live memory at that point
    (MB, rounded down). The stages:
      load_mb     the heap of the last phase before `taint` (ir, sites, graph, ...): the load stage, exact.
      summary_mb  the largest 'marked' value (the third number of `X->Y->Z MB`) of the collections that rivus did not
                  force, between that phase and `taint`: the summary stage. Not exact: Z contains what was
                  allocated while the mark ran, so it is an upper bound (README).
      after_mb    the heap of the `taint` phase (what is left when the summaries are dropped).
    Returns a dict (a value is None when the stage was not seen) with the phases and the number of collections."""
    phases, window, last_window, ngc = [], [], [], 0
    for line in text.splitlines():
        m = PHASE.match(line)
        if m:
            phases.append((m.group(1), int(m.group(2))))
            if m.group(1) == "taint":
                last_window = window
            window = []
            continue
        g = GCLINE.match(line)
        if g:
            ngc += 1
            if "(forced)" not in line:
                window.append(int(g.group(3)))
    names = [p[0] for p in phases]
    out = {"phases": dict(phases), "load_mb": None, "summary_mb": None, "after_mb": None, "gcs": ngc}
    if "taint" in names:
        i = names.index("taint")
        out["after_mb"] = phases[i][1]
        if i > 0:
            out["load_mb"] = phases[i - 1][1]
        out["summary_mb"] = max(last_window) if last_window else None
    return out


def growth(rows, keys):
    """For each pair of consecutive scales: the ratio of every key (value at the larger scale / at the smaller), and
    the exponent log(ratio)/log(scale ratio) (1 = linear, 2 = square). None when a value is missing or 0."""
    out = []
    for a, b in zip(rows, rows[1:]):
        g = {"from": a["scale"], "to": b["scale"]}
        for k in keys:
            x, y = a.get(k), b.get(k)
            if x and y and x > 0 and y > 0:
                r = y / x
                g[k] = {"ratio": round(r, 2), "exponent": round(math.log(r) / math.log(b["scale"] / a["scale"]), 2)}
            else:
                g[k] = None
        out.append(g)
    return out


def row_from(scale, report, stage, wall, cpu):
    st, su = report.get("stats", {}), report.get("summary", {})
    return {"scale": scale, "load_mb": stage["load_mb"], "summary_mb": stage["summary_mb"], "after_mb": stage["after_mb"],
            "functions": st.get("functions"), "summary_elems": st.get("summary_elems"), "summary_steps": st.get("summary_steps"),
            "wall_s": round(wall, 1), "cpu_s": round(cpu, 1), "memory_peak_bytes": su.get("memory_peak_bytes"),
            "gc_cpu_fraction": su.get("gc_cpu_fraction"), "live_heap_max_bytes": su.get("live_heap_max_bytes"),
            "converged": st.get("converged"), "passes": st.get("passes"), "gcs": stage["gcs"]}


def fmt(v, nd=0):
    return "-" if v is None else (("%.*f" % (nd, v)) if isinstance(v, float) else str(v))


GROW_KEYS = ("load_mb", "summary_mb", "functions", "summary_elems", "summary_steps", "wall_s", "cpu_s")


def table(rows, grows):
    cols = ("scale", "load_mb", "summary_mb", "functions", "summary_elems", "summary_steps", "wall_s", "cpu_s",
            "memory_peak_mb", "gc_cpu_fraction")
    lines = ["  ".join("%14s" % c for c in cols)]
    for r in rows:
        mp = r["memory_peak_bytes"] and r["memory_peak_bytes"] >> 20
        vals = [r["scale"], r["load_mb"], r["summary_mb"], r["functions"], r["summary_elems"], r["summary_steps"],
                r["wall_s"], r["cpu_s"], mp, r["gc_cpu_fraction"]]
        lines.append("  ".join("%14s" % fmt(v, 2 if isinstance(v, float) else 0) for v in vals))
    lines.append("")
    lines.append("growth between scales (ratio, exponent: 1 = linear in the scale, 2 = square):")
    for g in grows:
        parts = []
        for k in GROW_KEYS:
            x = g[k]
            parts.append("%s x%s (^%s)" % (k, x["ratio"], x["exponent"]) if x else "%s -" % k)
        lines.append("  %sx -> %sx: %s" % (g["from"], g["to"], "; ".join(parts)))
    return "\n".join(lines)


def run_scale(k, a, rivus, gen, tmp):
    target = os.path.join(tmp, "t%d" % k)
    subprocess.run([gen, "-out", target] + PROFILES[a.profile](k), check=True, stdout=subprocess.DEVNULL)
    env = dict(os.environ, GODEBUG="gctrace=1")
    cmd = [rivus, "analyze", "-v", "-workers", str(a.workers), "-no-deps", "-rules", "rules.yaml", "./..."]
    outp, errp = os.path.join(tmp, "o%d.json" % k), os.path.join(tmp, "e%d.txt" % k)
    with heavy_lock("scale-growth x%d" % k):
        r = d.run_watched(cmd, target, env, a.limit, int(a.cap_gb * GB), errp, outp)
    if r["why"]:
        sys.exit("scale-growth: x%d was stopped by the watchdog (%s) after %.0f s" % (k, r["why"], r["wall"]))
    if r["rc"] not in (0, 4):
        sys.exit("scale-growth: rivus failed at x%d (exit %s): %s" % (k, r["rc"], open(errp, errors="replace").read()[-800:]))
    stage = parse_stderr(open(errp, errors="replace").read())
    row = row_from(k, json.load(open(outp)), stage, r["wall"], r["cpu"])
    if r["rc"] == 4:
        row["incomplete"] = True
    return row


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[1])
    ap.add_argument("-scales", default="1,2,4", help="scales, comma separated (the first is the base)")
    ap.add_argument("-profile", default="mixed", choices=sorted(PROFILES))
    ap.add_argument("-workers", type=int, default=1, help="1 by default: less is allocated during a mark")
    ap.add_argument("-rivus", default="", help="the binary (default: built from this tree into a temporary directory)")
    ap.add_argument("-cap-gb", type=float, default=8.0, help="the watchdog: the sum of RSS of the tree (GB)")
    ap.add_argument("-limit", type=float, default=1800, help="the watchdog: wall seconds per run")
    a = ap.parse_args()
    scales = [int(x) for x in a.scales.split(",")]
    tmp = tempfile.mkdtemp(prefix="rivus-scale-growth-")
    try:
        rivus, gen = os.path.abspath(a.rivus) if a.rivus else os.path.join(tmp, "rivus"), os.path.join(tmp, "gen")
        if not a.rivus:
            subprocess.run(["go", "build", "-o", rivus, "./cmd/rivus"], cwd=ROOT, check=True)
        subprocess.run(["go", "build", "-o", gen, "./bench/go-scale/gen"], cwd=ROOT, check=True)
        rows = [run_scale(k, a, rivus, gen, tmp) for k in scales]
        grows = growth(rows, GROW_KEYS)
        print(table(rows, grows))
        commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
        out = os.path.join(d.out_dir(), "scale-growth-%s.json" % a.profile)
        json.dump({"commit": commit, "profile": a.profile, "workers": a.workers, "rows": rows, "growth": grows,
                   "load_avg": os.getloadavg()[0]}, open(out, "w"), indent=1)
        print("written: %s (load average now %.1f)" % (out, os.getloadavg()[0]))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    main()
