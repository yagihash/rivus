#!/usr/bin/env python3
"""Backtest of the bench: does the new bench go red on the bad versions, and stay green on the good ones?

  bench/backtest.py judge OUT.json            the red / green of one engine, per unit (from `go test -json`)
  bench/backtest.py table  OUTDIR  ROWS.json  the table of bench/BACKTEST.md: row x (bad, good) x unit, with the metric

The tests are the ones of the head (TestScaleSynth, TestCuratedGo, TestZooGo, the determinism tests) run with
RIVUS_BINARY=<an older build of cmd/rivus> (bench/backtest.sh): the inputs and the answer keys are the head's,
only the engine is old (bench/BACKTEST.md). A unit is:
  GREEN     the test passes
  RED       it fails; the reasons say which metric: FAKE (a flow into a sink that must stay quiet, or a flow that
            the key does not name), MISS (a flow of the key is not reported), NOCONV, SIZE (a size over the
            expected file by 10 percent or more, with the ratio), TRACE (the uncertainties by kind differ),
            FINDINGS (the set of the findings differs from the expected file)
  UNJUDGED  the old build could not read the input (a rules file in a newer format, an unknown flag): neither red
            nor green
"""
import json
import re
import sys
from collections import OrderedDict

GREW = re.compile(r"(\w+): (\w+) grew from (\d+) to (\d+)")


def reasons_of(lines):
    """The reasons of one failing unit, from the messages of its t.Error calls."""
    out = OrderedDict()
    fake = miss = 0
    for l in lines:
        l = l.strip()
        l = re.sub(r"^\w+_test\.go:\d+: ", "", l)
        m = GREW.search(l)
        if m:
            a, b = int(m.group(3)), int(m.group(4))
            out["SIZE %s x%.1f" % (m.group(2), b / a if a else 0)] = 1
        elif "must stay quiet" in l or l.startswith("unexpected flow"):
            fake += 1
        elif l.startswith("missing flow"):
            miss += 1
        elif "did not converge" in l or "converged was" in l:
            out["NOCONV"] = 1
        elif "uncertainties by kind" in l:
            out["TRACE"] = 1
        elif "findings are not the ones" in l:
            out["FINDINGS"] = 1
    r = []
    if fake:
        r.append("FAKE %d" % fake)
    if miss:
        r.append("MISS %d" % miss)
    r.extend(out.keys())
    return r


def judge(path):
    """{unit: (status, reasons)} for one `go test -json` output."""
    outs, status = {}, {}
    broken = False
    for raw in open(path, errors="replace"):
        try:
            e = json.loads(raw)
        except ValueError:
            continue
        t = e.get("Test")
        if e.get("Action") == "output":
            o = e.get("Output", "")
            if t:
                outs.setdefault(t, []).append(o)
            elif "cannot" in o or "panic" in o:
                broken = True
        elif e.get("Action") in ("pass", "fail", "skip") and t:
            status[t] = e["Action"]
    res = OrderedDict()
    for t in sorted(status):
        subs = [s for s in status if s.startswith(t + "/")]
        if subs:
            continue  # a parent: its children say it
        unit = t.replace("TestScaleSynth/", "scale/").replace("TestCuratedGo", "curated-go").replace("TestZooGo", "zoo-go")
        unit = unit.replace("TestDeterministic", "det/")
        text = "".join(outs.get(t, []))
        st = status[t]
        if st == "pass":
            res[unit] = ("GREEN", [])
        elif st == "skip":
            res[unit] = ("SKIP", [])
        else:
            if re.search(r"analyze: |unknown flag|rules?.*(error|unknown)|load rules|yaml:|unknown field", text) and "problems" not in text and not reasons_of(text.splitlines()):
                res[unit] = ("UNJUDGED", [text.strip().splitlines()[-1][:80] if text.strip() else "?"])
            else:
                res[unit] = ("RED", reasons_of(text.splitlines()))
    # a parent test (a profile of TestScaleSynth has no children): done above; keep a note if the run broke
    if broken and not res:
        res["(run)"] = ("UNJUDGED", ["the test binary did not run"])
    return res


def cell(res, unit):
    if unit not in res:
        return "-"
    st, r = res[unit]
    return st if not r else "%s (%s)" % (st, "; ".join(r))


OLD_RULES = """ruleset: rivus/backtest-det@1
sources:
  - symbol: example.com/cases/lib.Source
    labels: [user_input]
sinks:
  - symbol: example.com/cases/lib.Sink
    category: test
"""


def det(binary, runs):
    """The writers of one global from the same level (testdata/go/cases/paramshare/gwriters) must give the same
    output on every run, for every GOMAXPROCS and -workers (the time of the run is not part of the output). The
    rules are in the oldest format, so that every old build reads them. Prints DIFFERENT n/m or SAME m."""
    import os
    import subprocess
    import tempfile

    root = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    data = os.path.join(root, "testdata", "go")
    with tempfile.TemporaryDirectory() as tmp:
        rules = os.path.join(tmp, "rules.yaml")
        open(rules, "w").write(OLD_RULES)
        seen, total = {}, 0
        for procs in ("1", "4", ""):
            for workers in ("2", "16"):  # -workers 1 is another way through a big SCC (R-43), not what this row looks at
                for _ in range(runs):
                    env = dict(os.environ, RIVUS_CACHE_DIR=os.path.join(tmp, "cache"))
                    if procs:
                        env["GOMAXPROCS"] = procs
                    p = subprocess.run([binary, "analyze", "-workers", workers, "-rules", rules, "./cases/paramshare/gwriters"], cwd=data, capture_output=True, text=True, env=env)
                    if p.returncode != 0:
                        print("UNJUDGED (%s)" % p.stderr.strip().splitlines()[-1][:80])
                        return
                    rep = json.loads(p.stdout)
                    rep["stats"].pop("duration_ms", None)
                    for k in ("dep_spent_ms", "own_spent_ms"):
                        rep["stats"].pop(k, None)
                    key = json.dumps(rep, sort_keys=True)
                    seen[key] = seen.get(key, 0) + 1
                    total += 1
        if len(seen) == 1:
            print("SAME %d" % total)
        else:
            print("DIFFERENT: %d outputs in %d runs (the most common %d)" % (len(seen), total, max(seen.values())))


def main():
    if len(sys.argv) >= 3 and sys.argv[1] == "judge":
        for u, (st, r) in judge(sys.argv[2]).items():
            print("%-22s %-9s %s" % (u, st, "; ".join(r)))
    elif len(sys.argv) >= 3 and sys.argv[1] == "det":  # det BIN [runs]: the determinism row, with rules in the oldest format
        det(sys.argv[2], int(sys.argv[3]) if len(sys.argv) > 3 else 6)
    elif len(sys.argv) >= 4 and sys.argv[1] == "all":  # all OUTDIR ref...: what is not green, per engine
        for ref in sys.argv[3:]:
            print("== %s" % ref)
            for u, (st, r) in judge("%s/scale-%s.json" % (sys.argv[2], ref)).items():
                if st != "GREEN":
                    print("  %-22s %-9s %s" % (u, st, "; ".join(r)))
    elif len(sys.argv) >= 4 and sys.argv[1] == "table":
        outdir, rows = sys.argv[2], json.load(open(sys.argv[3]))
        cache = {}

        def get(ref):
            if ref not in cache:
                try:
                    cache[ref] = judge("%s/scale-%s.json" % (outdir, ref))
                except OSError:
                    cache[ref] = {}
            return cache[ref]

        for row in rows:
            print("### %s: %s" % (row["id"], row["what"]))
            print("| unit | bad %s | good %s |" % (row["bad"], row["good"]))
            print("|---|---|---|")
            for u in row["units"]:
                print("| %s | %s | %s |" % (u, cell(get(row["bad"]), u), cell(get(row["good"]), u)))
            print()
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main()
