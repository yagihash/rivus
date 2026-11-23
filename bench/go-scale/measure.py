#!/usr/bin/env python3
"""Measure rivus on a scale profile (bench/go-scale/gen) and score it against the answer key in the code.

  measure.py -rivus BIN [-shapes types,stderr|all|none] [-workers 1] [-gen "-funcs 3000"] [-timeout 900s] [-json]

Generates the module of the shapes (the same bytes every time), runs `rivus analyze -workers N` on it, and
prints one line: functions, passes, converged, reanalyses, summary_elems, the largest summary, time, and the score:
  found    flows of the key (@source(tag) -> @sink(tag)) that are reported
  missing  flows of the key that are not reported
  false    reported pairs that the key does not name (a finding into @sink(none), or into an @nosink line, or any other)
The key is read in the way of cmd/rivus/bench_curated_test.go. The sizes are the engine's own numbers: compare two
builds on the same profile (bench/README.md, "The scale profiles"); the test TestScaleSynth keeps the head's numbers.
"""
import argparse, json, os, re, subprocess, sys, tempfile, time, shutil

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
from heavylock import heavy_lock  # one lock for the heavy runs of the whole machine (bench/heavylock.py)

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
ANNOT = re.compile(r"@(source|sink|nosink)\(([A-Za-z0-9_-]+)\)")


def key(d):
    src, sinks, none, nos = {}, {}, set(), []
    for dp, _, fs in os.walk(d):
        for f in fs:
            if not f.endswith(".go"):
                continue
            path = os.path.join(dp, f)
            rel = os.path.relpath(path, d)
            for n, line in enumerate(open(path), 1):
                for kind, tag in ANNOT.findall(line):
                    if kind == "source":
                        src[tag] = (rel, n)
                    elif kind == "sink" and tag == "none":
                        none.add((rel, n))
                    elif kind == "sink":
                        sinks.setdefault(tag, []).append((rel, n))
                    else:
                        nos.append((tag, (rel, n)))
    want = {(src[t], k) for t, ks in sinks.items() if t in src for k in ks}
    return want, none, {(src[t], k) for t, k in nos if t in src}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-rivus", required=True)
    ap.add_argument("-shapes", default="types")
    ap.add_argument("-workers", type=int, default=1)
    ap.add_argument("-gen", default="")
    ap.add_argument("-timeout", default="900s")
    ap.add_argument("-json", action="store_true")
    a = ap.parse_args()
    tmp = tempfile.mkdtemp(prefix="rivus-scale-")
    try:
        d = os.path.join(tmp, "m")
        subprocess.run(["go", "run", "./bench/go-scale/gen", "-shapes", a.shapes, "-out", d] + a.gen.split(), cwd=ROOT, check=True)
        with heavy_lock("go-scale measure"):
            t0 = time.time()
            p = subprocess.run([os.path.abspath(a.rivus), "analyze", "-sites", "-workers", str(a.workers), "-timeout", a.timeout, "-rules", "rules.yaml", "./..."], cwd=d, capture_output=True, text=True)
        wall = time.time() - t0
        if p.returncode == 4:  # exitIncomplete: the output is written; the key check below judges it
            print("go-scale: the analysis is not complete (exit code 4)", file=sys.stderr)
        elif p.returncode != 0:
            sys.exit("rivus failed: %s\n%s" % (p.returncode, p.stderr[-1500:]))
        rep = json.loads(p.stdout)
        want, none, nos = key(d)
        got = set()
        for f in rep["findings"]:
            k = (f["sink"]["position"]["file"], f["sink"]["position"]["line"])
            for s in [f["source"]] + f.get("other_sources", []):
                got.add(((s["position"]["file"], s["position"]["line"]), k))
        st = rep["stats"]
        out = {"shapes": a.shapes, "functions": st["functions"], "passes": st["passes"], "converged": st["converged"],
               "reanalyses": st["reanalyses"], "summary_elems": st["summary_elems"],
               "largest_summary_elems": st.get("largest_summary_elems", 0), "wall_s": round(wall, 1),
               "found": len(want & got), "missing": len(want - got), "false": len(got - want)}
        if a.json:
            print(json.dumps(out, sort_keys=True))
        else:
            print("%-22s funcs %5d passes %2d conv %-5s reanal %6d elems %7d maxsum %4d | found %d missing %d false %d | %.1fs" % (
                a.shapes, out["functions"], out["passes"], out["converged"], out["reanalyses"], out["summary_elems"],
                out["largest_summary_elems"], out["found"], out["missing"], out["false"], wall))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    main()
