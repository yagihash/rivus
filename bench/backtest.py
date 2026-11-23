#!/usr/bin/env python3
"""Backtest of the bench: does the new bench go red on the bad versions, and stay green on the good ones?

  bench/backtest.py judge OUT.json            the red / green of one engine, per unit (from `go test -json`)
  bench/backtest.py table  OUTDIR  ROWS.json  row x (bad, good) x unit, with the metric (the raw judgement of each unit)
  bench/backtest.py docs   [-w] ROWS.json BACKTEST.md LEDGER_HEADER.md
                                              the table of bench/BACKTEST.md and the head of bench/LEDGER.md are made from
                                              bench/backtest-rows.json (the only source of the rows); -w writes them, without
                                              -w it fails when a file is not the one that the rows make

The tests are the ones of the head (TestScaleSynth, TestCuratedGo, TestZooGo, the determinism tests) run with
RIVUS_BINARY=<an older build of cmd/rivus> (how: bench/BACKTEST.md, "How it is run"): the inputs and the answer keys are the head's,
only the engine is old (bench/BACKTEST.md). A unit is:
  GREEN     the test passes
  RED       it fails; the reasons say which metric: FAKE (a flow into a sink that must stay quiet, or a flow that
            the key does not name), MISS (a flow of the key is not reported), NOCONV, SIZE (a size over the
            expected file by 10 percent or more, with the ratio), TRACE (the uncertainties by kind differ), OTHER_SOURCES (the count of other sources of the findings),
            FINDINGS (the set of the findings differs from the expected file)
  UNJUDGED  the old build could not read the input (a rules file in a newer format, an unknown flag): neither red
            nor green
  ERROR     it fails, and the reason is not one of the above (a crash, a message that this tool does not know): it is
            counted, never dropped; read the last line of the output that is printed with it

The exit status (judge, all, table, det) is 1 when a unit is RED or ERROR (det: DIFFERENT), 2 when nothing could be
judged (the binary does not run, UNJUDGED only), else 0. A bad version is expected to be red: the caller reads it.
"""
import json
import os
import re
import sys
from collections import OrderedDict

# an old build that cannot read an input of the head (a rules file in a newer format, a flag it does not have)
CANNOT_READ = re.compile(r"unknown flag|flag provided but not defined|load rules|yaml:|unknown field|rules?.*(error|unknown)")
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
        elif "other_sources_count was" in l:
            out["OTHER_SOURCES"] = 1
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
            reasons = reasons_of(text.splitlines())
            last = (text.strip().splitlines() or ["(no output)"])[-1][:100]
            if reasons:
                res[unit] = ("RED", reasons)
            elif CANNOT_READ.search(text):
                res[unit] = ("UNJUDGED", [last])
            else:
                res[unit] = ("ERROR", ["unrecognized: " + last])
    # a parent test (a profile of TestScaleSynth has no children): done above; keep a note if the run broke
    if not res:
        res["(run)"] = ("ERROR", ["the test binary did not run" if broken else "no test result in the output"])
    return res


ROWS_BEGIN = "<!-- rows:begin (made by `bench/backtest.py docs -w` from bench/backtest-rows.json; do not edit) -->"
ROWS_END = "<!-- rows:end -->"


def rows_table(rows):
    """The table of bench/BACKTEST.md, from the rows (the only source)."""
    out = ["| # | what | bad | good | layer | red by (bad) | good |", "| --- | --- | --- | --- | --- | --- | --- |"]
    for r in rows:
        out.append("| %s | %s | %s | %s | %s | %s | %s |" % (r["id"], r["what"], r["bad"], r["good"], r["layer"], r["red_by"], r["good_result"]))
    return "\n".join(out)


def ledger_header(rows):
    """The head of bench/LEDGER.md: the count of the unverified rows, from the rows."""
    counted = [r for r in rows if r["status"] in ("verified", "unverified")]
    unv = [r for r in counted if r["status"] == "unverified"]
    out = ["**Unverified rows: %d** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main; "
           "the status of each row is in `bench/backtest-rows.json`). It is counted again at every change of that file." % len(unv), ""]
    if unv:
        out += ["| row | why it is unverified | what makes it verified |", "|---|---|---|"]
        for r in unv:
            out.append("| %s (%s) | %s | %s |" % (r["id"], r["what"], r.get("unverified_why", "-"), r.get("verified_when", "-")))
    else:
        out.append("No row is unverified.")
    out.append("")
    for r in rows:
        if r.get("history"):
            out.append("Row %s: %s." % (r["id"], r["history"]))
    out += ["", "Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.", ""]
    days = [r["verified_on"] for r in counted if r.get("verified_on")]
    if unv:
        out.append("**The day the number reached 0: not yet** (the last row to be verified: %s)." % (max(days) if days else "none"))
    else:
        out.append("**The day the number reached 0: %s** (the day the last of the rows was verified; a row that is added later starts as unverified)." % max(days))
    out += ["", "Holes: only the pairs of rows 6, 7 (layer C) have compare runs so far, and row 8 and 10 are judged by `TestCuratedGo`; the rows of A (go-scale) are in BACKTEST.md. "
            "Wall times were taken while the machine was shared (read them as a guide; the counts are exact)."]
    return "\n".join(out) + "\n"


def splice(text, table):
    a, b = text.find("<!-- rows:begin"), text.find(ROWS_END)
    if a < 0 or b < a:
        raise SystemExit("backtest: the markers of the table are not in the file")
    return text[:a] + ROWS_BEGIN + "\n" + table + "\n" + text[b:]


def docs(args):
    write = False
    if args and args[0] == "-w":
        write, args = True, args[1:]
    rows_path, bt_path, hdr_path = args
    rows = json.load(open(rows_path))
    ledger_path = os.path.join(os.path.dirname(os.path.abspath(hdr_path)), "LEDGER.md")
    want = {bt_path: splice(open(bt_path).read(), rows_table(rows)), hdr_path: ledger_header(rows)}
    led = open(ledger_path).read()
    i = led.find("# Ledger: version")
    # bench/model-ledger.md: the counts of the rows of the models that the differential test judged (written by hand from the output of
    # `go run ./bench/model-diff -dead-allow <dir>`, with the day); it goes between the head and the table of versions
    extra_path = os.path.join(os.path.dirname(os.path.abspath(hdr_path)), "model-ledger.md")
    extra = open(extra_path).read().strip("\n") if os.path.exists(extra_path) else ""
    want[ledger_path] = ledger_header(rows).rstrip("\n") + "\n\n" + (extra + "\n\n" if extra else "") + (led[i:] if i >= 0 else "")
    bad = []
    for path, text in want.items():
        if open(path).read() != text:
            bad.append(path)
            if write:
                open(path, "w").write(text)
    if bad and not write:
        print("not the ones that bench/backtest-rows.json makes (run `python3 bench/backtest.py docs -w ...`): " + ", ".join(bad))
        return 1
    return 0


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
                        lines = (p.stderr.strip() or p.stdout.strip()).splitlines()
                        last = lines[-1][:80] if lines else "exit status %d, no output" % p.returncode  # a crash may have no stderr at all
                        if CANNOT_READ.search(p.stderr):
                            print("UNJUDGED (%s)" % last)
                            return 2
                        print("ERROR (unrecognized: %s)" % last)
                        return 1
                    try:
                        rep = json.loads(p.stdout)
                        rep["stats"]
                    except (ValueError, KeyError, TypeError):
                        print("ERROR (unrecognized: the output is not a report)")
                        return 1
                    rep["stats"].pop("duration_ms", None)
                    for k in ("dep_spent_ms", "own_spent_ms"):
                        rep["stats"].pop(k, None)
                    key = json.dumps(rep, sort_keys=True)
                    seen[key] = seen.get(key, 0) + 1
                    total += 1
        if len(seen) == 1:
            print("SAME %d" % total)
            return 0
        print("DIFFERENT: %d outputs in %d runs (the most common %d)" % (len(seen), total, max(seen.values())))
        return 1


def exit_code(statuses):
    """1 when a unit is RED or ERROR; 2 when nothing could be judged (every unit UNJUDGED or SKIP); else 0."""
    st = list(statuses)
    if any(x in ("RED", "ERROR") for x in st):
        return 1
    if st and all(x in ("UNJUDGED", "SKIP") for x in st) and "UNJUDGED" in st:
        return 2
    return 0


def main():
    a = sys.argv
    if len(a) >= 3 and a[1] == "judge":
        res = judge(a[2])
        for u, (st, r) in res.items():
            print("%-22s %-9s %s" % (u, st, "; ".join(r)))
        return exit_code(st for st, _ in res.values())
    if len(a) >= 3 and a[1] == "det":  # det BIN [runs]: the determinism row, with rules in the oldest format
        return det(a[2], int(a[3]) if len(a) > 3 else 6)
    if len(a) >= 4 and a[1] == "all":  # all OUTDIR ref...: what is not green, per engine
        codes = []
        for ref in a[3:]:
            print("== %s" % ref)
            try:
                res = judge("%s/scale-%s.json" % (a[2], ref))
            except OSError as e:
                print("  ERROR (cannot read the judgement: %s)" % e)
                codes.append(1)
                continue
            for u, (st, r) in res.items():
                if st != "GREEN":
                    print("  %-22s %-9s %s" % (u, st, "; ".join(r)))
            codes.append(exit_code(st for st, _ in res.values()))
        return max(codes, default=0)
    if len(a) >= 5 and a[1] == "docs":
        return docs(a[2:])
    if len(a) >= 4 and a[1] == "table":
        outdir, rows = a[2], json.load(open(a[3]))
        cache, sts = {}, []

        def get(ref):
            if ref not in cache:
                try:
                    cache[ref] = judge("%s/scale-%s.json" % (outdir, ref))
                except OSError:
                    cache[ref] = {}
            return cache[ref]

        for row in rows:
            print("### %s: %s" % (row["id"], row["what"]))
            print("| unit | bad %s | good %s |" % (row["bad_ref"], row["good_ref"]))
            print("|---|---|---|")
            for u in row["units"]:
                print("| %s | %s | %s |" % (u, cell(get(row["bad_ref"]), u), cell(get(row["good_ref"]), u)))
                sts += [get(row["bad_ref"]).get(u, ("-",))[0], get(row["good_ref"]).get(u, ("-",))[0]]
            print()
        return exit_code(sts)
    sys.exit(__doc__)


if __name__ == "__main__":
    sys.exit(main())
