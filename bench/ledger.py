#!/usr/bin/env python3
"""The ledger of the bench (bench/LEDGER.md): version x bench x metric.

  bench/ledger.py -versions bench/ledger-versions.tsv -ref de627ce [-deps bench/deps-ledger.tsv] [-passline bench/passline-baseline.md] RESULTS_DIR... > bench/LEDGER.md

RESULTS_DIR is a directory that bench/compare.sh wrote (--out, or <work>/results/<pair>): its raw/ holds
<bench>.<side>.json, .meta.json and .prov. A run is named by the engine that made it (the 4th field of the .prov
file, a git short hash) and by the inputs (a file `fixed-inputs` in the results directory says that the inputs
were the ones of the tree the script was in, bench/BACKTEST.md; otherwise each side had its own).

Per bench, one row per (engine, inputs): findings, the difference against the reference engine on the same
inputs (+added -removed, by sink file / line / category and the source: only the numbers are printed, never a
place: the third-party code of the corpora is not described), converged, passes, reanalyses, summary_elems, the
largest summary, wall seconds, the peak memory, the biggest kinds of uncertainty. "-" is a hole: the pair was not
run (the ledger is made of what was run, not of what was assumed).
"""
import argparse
import glob
import json
import os
import sys


def load_versions(path):
    v = {}
    if path and os.path.exists(path):
        for line in open(path):
            line = line.rstrip("\n")
            if not line or line.startswith("#"):
                continue
            f = line.split("\t")
            v[f[0]] = (f[1] if len(f) > 1 else "", f[2] if len(f) > 2 else "")
    return v


def finding_keys(rep):
    ks = set()
    for f in rep.get("findings", []):
        s, k = f["source"]["position"], f["sink"]["position"]
        ks.add((s["file"], s["line"], k["file"], k["line"], f.get("category", "")))
    return ks


def read_run(rawdir, bench, side):
    base = os.path.join(rawdir, "%s.%s" % (bench, side))
    try:
        rep = json.load(open(base + ".json"))
        prov = open(base + ".prov").read().rstrip("\n").split("\t")
    except (OSError, ValueError):
        return None
    meta = {}
    try:
        meta = json.load(open(base + ".meta.json"))
    except (OSError, ValueError):
        pass
    st = rep.get("stats", {})
    kinds = {}
    for u in rep.get("uncertainties", []):
        kinds[u["kind"]] = kinds.get(u["kind"], 0) + 1
    return {
        "engine": prov[3] if len(prov) > 3 else "?",
        "keys": finding_keys(rep),
        "n": len(rep.get("findings", [])),
        "conv": st.get("converged"),
        "passes": st.get("passes"),
        "reanalyses": st.get("reanalyses"),
        "elems": st.get("summary_elems"),
        "maxsum": st.get("largest_summary_elems"),
        "wall": meta.get("wall_s"),
        "rss": meta.get("max_rss_mb"),
        "kinds": kinds,
        "rc": meta.get("rc"),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-versions", default="")
    ap.add_argument("-ref", default="")
    ap.add_argument("-header", default="", help="a file whose text is printed first (the unverified rows and the date they reached 0)")
    ap.add_argument("-deps", default="", help="bench/deps-ledger.tsv (bench/deps_bench.py, `make bench-deps`): printed last as its own section")
    ap.add_argument("-passline", default="", help="bench/passline-baseline.md (printed as is) and, from -passline-ledger, the lines of `make bench-passline LINE_LEDGER=1`")
    ap.add_argument("-passline-ledger", default="bench/passline-ledger.tsv")
    ap.add_argument("dirs", nargs="+")
    a = ap.parse_args()
    labels = load_versions(a.versions)
    runs = {}  # bench -> {(engine, inputs): run}
    for d in a.dirs:
        raw = os.path.join(d, "raw")
        inputs = "head" if os.path.exists(os.path.join(d, "fixed-inputs")) or os.path.exists(os.path.join(raw, "fixed-inputs")) else "own"
        for p in glob.glob(os.path.join(raw, "*.base.json")) + glob.glob(os.path.join(raw, "*.head.json")):
            name = os.path.basename(p)[: -len(".json")]
            bench, side = name.rsplit(".", 1)
            r = read_run(raw, bench, side)
            if r:
                old = runs.setdefault(bench, {}).get((r["engine"], inputs))
                if old is not None:
                    same = "the findings are the same" if old["keys"] == r["keys"] else "THE FINDINGS DIFFER (%d and %d)" % (old["n"], r["n"])
                    print("ledger: WARNING: %s, engine %s, inputs %s was already read from %s; the run in %s replaces it (the later directory wins; %s)" % (
                        bench, r["engine"], inputs, old["dir"], d, same), file=sys.stderr)
                r["dir"] = d
                runs[bench][(r["engine"], inputs)] = r
    if a.header:
        print(open(a.header).read().rstrip("\n") + "\n")
    print("# Ledger: version x bench x metric\n")
    print("Made by `bench/ledger.py` from the runs of `bench/compare.sh` (see the header of that script). Inputs `head`: the engine of the row was run on the inputs of the reference tree (`compare.sh --fixed-inputs`, bench/BACKTEST.md); `own`: on the inputs of its own version.")
    print("The difference is against the reference engine `%s` on the same inputs: `+added -removed` findings (counts only). A `-` is a hole: not run.\n" % a.ref)
    for bench in sorted(runs):
        rs = runs[bench]
        ref = {}
        for (e, inp), r in rs.items():
            if e == a.ref:
                ref[inp] = r
        print("## %s\n" % bench)
        print("| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |")
        print("|---|---|---|---|---|---|---|---|---|---|---|---|---|")
        for (e, inp), r in sorted(rs.items(), key=lambda kv: (kv[0][1], kv[0][0])):
            rr = ref.get(inp)
            if rr is None or e == a.ref:
                diff = "ref" if e == a.ref else "-"
            else:
                diff = "+%d -%d" % (len(r["keys"] - rr["keys"]), len(rr["keys"] - r["keys"]))
            top = sorted(r["kinds"].items(), key=lambda kv: -kv[1])[:3]
            lab = labels.get(e, ("", ""))
            print("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |" % (
                e, lab[1] or lab[0], inp, r["n"], diff, "yes" if r["conv"] else ("NO" if r["conv"] is False else "-"),
                r["passes"], r["reanalyses"], r["elems"], r["maxsum"], r["wall"], r["rss"],
                ", ".join("%s %d" % kv for kv in top) or "-"))
        print()
    if a.deps and os.path.exists(a.deps):
        print_deps(a.deps)
    if a.passline and os.path.exists(a.passline):
        print_passline(a.passline, a.passline_ledger)


def print_deps(path):
    rows = [l.rstrip("\n").split("\t") for l in open(path) if l.strip()]
    print("## Benches with heavy dependencies (`make bench-deps`)\n")
    print("`rivus analyze` with no flag that changes the analysis, on a local run (bench/deps_bench.py; the columns are described there). One row per run; compare the same bench between dates. `finished`: yes / timeout / oom / error. `-` is not measured (max RSS of a run that we killed). The engine is the commit the binary was built from.\n")
    print("| " + " | ".join(rows[0]) + " |")
    print("|" + "---|" * len(rows[0]))
    for r in rows[1:]:
        print("| " + " | ".join(r) + " |")
    print()


def print_passline(baseline, ledger):
    print(open(baseline).read().rstrip("\n") + "\n")
    if os.path.exists(ledger):
        rows = [l.rstrip("\n").split("\t") for l in open(ledger) if l.strip()][1:]
        if rows:
            print("### Lines measured so far\n")
            print("| date | rivus | line |\n|---|---|---|")
            for r in rows:
                print("| " + " | ".join(r) + " |")
            print()


if __name__ == "__main__":
    main()
