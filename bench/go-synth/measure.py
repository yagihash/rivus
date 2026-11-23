#!/usr/bin/env python3
"""Measure rivus on a generated module: time, peak memory, findings, uncertainties.

  measure.py run -rivus BIN -size S [-runs 3] [-out result.json] [-flags "-timeout 900s"]
  measure.py run -rivus BIN -dir DIR [-runs 3]      (any module that has rules.yaml)
  measure.py run -lang dart|rust ...                (the generator is bench/<lang>-synth/gen; the extractor
                                                     writes the IR once, and only "rivus analyze -ir" is measured:
                                                     -dart "mise exec dart@3.13.6 -- dart" or $RIVUS_DART,
                                                     -rust PATH or $RIVUS_RUST)
  every command takes -lang (go by default); expected/ is bench/<lang>-synth/expected
  measure.py compare before.json after.json
  measure.py check -rivus BIN -size S               (is the result the one in expected/S.json?)
  measure.py snapshot -rivus BIN -size S            (write expected/S.json: only when the result is meant to change)

"run" writes (and prints) one JSON object: the wall time and peak memory of each
run, and, of the first run, the counts and the hash of the set of finding IDs.
"compare" says whether the findings are identical and how the time and the memory
changed. Only the findings, uncertainties and ID hash decide "identical"; time and
memory are for reading, and they move between runs (see README.md).
"""
import argparse, shlex, hashlib, json, os, resource, shutil, subprocess, sys, tempfile, time

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(HERE, ".."))
from heavylock import heavy_lock  # one lock for the heavy runs of the whole machine (bench/heavylock.py)


def generate(size, out, lang="go"):
    subprocess.run(["go", "run", "./bench/%s-synth/gen" % lang, "-size", size, "-out", out], cwd=ROOT, check=True)


def extract(lang, d, a):
    """Writes the IR of the project in d with the extractor of the language; returns (path, seconds)."""
    ir = os.path.join(d, "ir.jsonl")
    if lang == "dart":
        words = shlex.split(a.dart or os.environ.get("RIVUS_DART", "") or "dart")
        cmd = words + ["run", "bin/rivus_dart.dart", d]
        cwd = os.path.join(ROOT, "extractor", "dart")
    else:
        binary = a.rust or os.environ.get("RIVUS_RUST", "") or os.path.join(ROOT, "extractor", "rust", "target", "debug", "rivus-rust")
        cmd = [os.path.abspath(binary), d]
        cwd = ROOT
    t0 = time.time()
    with open(ir, "wb") as f:
        r = subprocess.run(cmd, cwd=cwd, stdout=f, stderr=subprocess.PIPE)
    if r.returncode != 0:
        sys.exit("the extractor failed: %s\n%s" % (r.returncode, r.stderr.decode()[-2000:]))
    return ir, time.time() - t0


def one(rivus, d, extra):
    with heavy_lock("go-synth measure"):
        return _one(rivus, d, extra)


def _one(rivus, d, extra):
    cmd = [rivus, "analyze", "-v", "-rules", os.path.join(d, "rules.yaml")] + extra
    if "-ir" not in extra:
        cmd.append("./...")
    t0 = time.time()
    p = subprocess.Popen(cmd, cwd=d, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out, err = p.communicate()
    # communicate() does not give the rusage of the child; wait4 does, but the
    # process is gone already: use the children's total, which is this one
    # when it is the only child that ran (the caller starts a fresh process per run).
    wall = time.time() - t0
    ru = resource.getrusage(resource.RUSAGE_CHILDREN)
    rss = ru.ru_maxrss / (1 if sys.platform == "darwin" else 1024)  # bytes on macOS, KB on Linux
    if p.returncode != 0:
        sys.exit("rivus failed: %s\n%s" % (p.returncode, err.decode()[-2000:]))
    rep = json.loads(out)
    phases = {}
    for line in err.decode().splitlines():
        parts = line.split()
        if line.startswith("rivus:") and len(parts) >= 3 and parts[2][-1] == "s":
            phases[parts[1]] = parts[2]
    return wall, rss / 1048576, rep, phases


def summarize(rep):
    ids = sorted(f["id"] for f in rep["findings"])
    kinds = {}
    for u in rep["uncertainties"]:
        kinds[u["kind"]] = kinds.get(u["kind"], 0) + 1
    st = rep.get("stats", {})
    return {
        "findings": len(ids),
        "uncertainties": len(rep["uncertainties"]),
        "uncertainty_kinds": dict(sorted(kinds.items())),
        "ids_sha256": hashlib.sha256("\n".join(ids).encode()).hexdigest(),
        "uncertainties_sha256": hashlib.sha256(
            "\n".join(sorted(json.dumps(u, sort_keys=True) for u in rep["uncertainties"])).encode()).hexdigest(),
        "stats": {k: st.get(k) for k in ("functions", "passes", "converged", "levels", "largest_scc", "widest_level", "reanalyses", "summary_elems", "summary_steps")},
    }


def run(a, quiet=False):
    d = a.dir
    tmp = None
    extra = list(a.extra)
    extract_s = None
    if not d:
        tmp = tempfile.mkdtemp(prefix="rivus-synth-")
        d = os.path.join(tmp, a.size)
        generate(a.size, d, a.lang)
    if a.lang != "go":
        ir, extract_s = extract(a.lang, os.path.abspath(d), a)
        extra += ["-ir", ir]
    rivus = os.path.abspath(a.rivus)
    runs = []
    first = None
    for i in range(a.runs):
        # a fresh interpreter per run, so that the peak memory is the run's own
        r = subprocess.run([sys.executable, __file__, "_one", rivus, d] + extra, capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit(r.stderr)
        o = json.loads(r.stdout)
        runs.append({"wall_s": round(o["wall"], 2), "max_rss_mb": round(o["rss"]), "phases": o["phases"]})
        if first is None:
            first = o["summary"]
        elif o["summary"]["ids_sha256"] != first["ids_sha256"] or o["summary"]["uncertainties_sha256"] != first["uncertainties_sha256"]:
            sys.exit("the result differs between runs of the same binary")
    res = {"lang": a.lang, "size": a.size if not a.dir else a.dir, "binary": rivus, "runs": runs, **first}
    if extract_s is not None:
        res["extract_s"] = round(extract_s, 1)  # the extractor, once; not what is measured
    w = sorted(r["wall_s"] for r in runs)
    res["wall_s_min"], res["wall_s_max"] = w[0], w[-1]
    res["max_rss_mb_max"] = max(r["max_rss_mb"] for r in runs)
    if tmp:
        shutil.rmtree(tmp, ignore_errors=True)
    s = json.dumps(res, indent=2, sort_keys=True)
    if a.out:
        open(a.out, "w").write(s + "\n")
    if not quiet:
        print(s)
    return res


EXPECTED_KEYS = ("findings", "uncertainties", "uncertainty_kinds", "ids_sha256", "uncertainties_sha256")


def expected_path(size, lang="go"):
    return os.path.join(ROOT, "bench", lang + "-synth", "expected", size + ".json")


def snapshot(a):
    a.runs = 1
    a.dir = None
    a.out = None
    res = run(a, quiet=True)
    keep = {k: res[k] for k in EXPECTED_KEYS}
    keep["stats"] = {k: res["stats"][k] for k in ("functions", "passes", "converged", "largest_scc", "levels")}  # not the sizes of the summaries: they are the engine's own
    os.makedirs(os.path.dirname(expected_path(a.size, a.lang)), exist_ok=True)
    open(expected_path(a.size, a.lang), "w").write(json.dumps(keep, indent=2, sort_keys=True) + "\n")
    print("wrote", expected_path(a.size, a.lang))


def check(a):
    a.runs = 1
    a.dir = None
    a.out = None
    res = run(a, quiet=True)
    want = json.load(open(expected_path(a.size, a.lang)))
    bad = [k for k in EXPECTED_KEYS if res[k] != want[k]]
    bad += ["stats." + k for k in want["stats"] if res["stats"][k] != want["stats"][k]]
    if bad:
        print("DIFFERENT from %s-synth/expected/%s.json: %s" % (a.lang, a.size, ", ".join(bad)))
        print("findings %s -> %s, uncertainties %s -> %s" % (want["findings"], res["findings"], want["uncertainties"], res["uncertainties"]))
        sys.exit(1)
    print("identical to %s-synth/expected/%s.json (findings %d, uncertainties %d); %.1fs, %d MB" % (
        a.lang, a.size, res["findings"], res["uncertainties"], res["wall_s_min"], res["max_rss_mb_max"]))


def compare(a):
    b, c = json.load(open(a.before)), json.load(open(a.after))
    same = all(b[k] == c[k] for k in ("findings", "uncertainties", "ids_sha256", "uncertainties_sha256"))
    print("results: %s (findings %d -> %d, uncertainties %d -> %d)" % (
        "IDENTICAL" if same else "DIFFERENT", b["findings"], c["findings"], b["uncertainties"], c["uncertainties"]))
    print("time:    %.1fs-%.1fs -> %.1fs-%.1fs (min of runs x%.2f)" % (
        b["wall_s_min"], b["wall_s_max"], c["wall_s_min"], c["wall_s_max"], c["wall_s_min"] / b["wall_s_min"]))
    print("memory:  %d MB -> %d MB" % (b["max_rss_mb_max"], c["max_rss_mb_max"]))
    sys.exit(0 if same else 1)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "_one":
        wall, rss, rep, phases = one(sys.argv[2], sys.argv[3], sys.argv[4:])
        print(json.dumps({"wall": wall, "rss": rss, "phases": phases, "summary": summarize(rep)}))
        sys.exit(0)
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("run")
    r.add_argument("-rivus", required=True)
    r.add_argument("-size", default="S")
    r.add_argument("-dir")
    r.add_argument("-runs", type=int, default=1)
    r.add_argument("-out")
    r.add_argument("-flags", default="", help="more flags for rivus analyze, as one string (\"-timeout 900s\")")
    for x in [r] + [sub.add_parser(n) for n in ("snapshot", "check")]:
        x.add_argument("-lang", default="go", choices=["go", "dart", "rust"])
        x.add_argument("-dart", default="", help="the command that runs dart (default $RIVUS_DART)")
        x.add_argument("-rust", default="", help="the Rust extractor (default $RIVUS_RUST)")
    for name in ("snapshot", "check"):
        x = sub.choices[name]
        x.add_argument("-rivus", required=True)
        x.add_argument("-size", default="S")
        x.add_argument("-flags", default="")
    c = sub.add_parser("compare")
    c.add_argument("before")
    c.add_argument("after")
    a = ap.parse_args()
    a.extra = shlex.split(getattr(a, "flags", ""))
    {"run": run, "compare": compare, "snapshot": snapshot, "check": check}[a.cmd](a)
