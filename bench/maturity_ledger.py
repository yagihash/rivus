#!/usr/bin/env python3
"""The ledger of maturity: four numbers per language, one row each, added to bench/maturity-ledger.tsv.

  python3 bench/maturity_ledger.py [--dry-run] [--no-edges] [--ledger FILE]      (make maturity-ledger)

todo, miss: the zoo test of each language (`go test ./cmd/rivus -run 'TestZoo(Go|Dart|Rust)$' -v`).
no_edge: Go: `go run ./bench/observed-edges run -zoo bench/zoo-go`; Dart: bench/zoo-dart/observed/compare.py (needs `dart`).
call_mismatch: empty until the check of the call counts exists (docs/ROADMAP.md, B(3)).
An input that cannot be had leaves its column empty, and the reason goes to standard error: an empty cell is not a zero.
What a number does not show is in bench/README.md (the section "The ledger of maturity").
The script only adds rows. The commit is made by a person. Standard library only.
"""
import argparse, datetime, os, re, shutil, subprocess, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LEDGER = os.path.join(ROOT, "bench", "maturity-ledger.tsv")
COLUMNS = ["date", "rivus_commit", "lang", "todo", "miss", "no_edge", "call_mismatch"]
LANGS = ["go", "dart", "rust"]

MARKS_RE = re.compile(r"zoo-(go|dart|rust): parity marks: @na (\d+) \[([^\]]*)\] \| @todo (\d+) \[([^\]]*)\]")
MISS_RE = re.compile(r"zoo-(go|dart|rust): cells \d+ / cases \d+ \| found \d+ / \d+ \| @miss (\d+) \(trace \d+, silent \d+, no rule \d+\)")
NO_EDGE_RE = re.compile(r"^\s*no_edge\s+(\d+)\s*$", re.M)
COMPARE_RE = re.compile(r"^\w+\s+no_edge\s+(\d+)\s*$", re.M)


def parse_todo(text):
    """{lang: @todo count} from the "parity marks" lines of the zoo tests."""
    return {m.group(1): int(m.group(4)) for m in MARKS_RE.finditer(text)}


def parse_miss(text):
    """{lang: @miss count} from the "cells .. | found .. | @miss N (trace ..)" lines of the zoo tests."""
    return {m.group(1): int(m.group(2)) for m in MISS_RE.finditer(text)}


def parse_no_edge_go(text):
    """The count of `no_edge` in the report of bench/observed-edges (a line "  no_edge      N"), or None."""
    m = NO_EDGE_RE.search(text)
    return int(m.group(1)) if m else None


def parse_no_edge_dart(text):
    """The count of `no_edge` in the summary of bench/zoo-dart/observed/compare.py (lines "call  no_edge  N"), or None."""
    ms = COMPARE_RE.findall(text)
    return sum(int(x) for x in ms) if ms else (0 if "distinct observed edges" in text else None)


def make_rows(date, commit, values):
    """values: {lang: {column: int or None}}. One row (a list of strings) per language, in the order of LANGS."""
    rows = []
    for lang in LANGS:
        v = values.get(lang, {})
        rows.append([date, commit, lang] + ["" if v.get(c) is None else str(v[c]) for c in COLUMNS[3:]])
    return rows


def existing_keys(text):
    keys = set()
    for line in text.splitlines()[1:]:
        f = line.split("\t")
        if len(f) >= 3:
            keys.add((f[0], f[1], f[2]))
    return keys


def new_rows(rows, text):
    """The rows that the ledger does not have yet (same date, commit and language)."""
    have = existing_keys(text)
    return [r for r in rows if (r[0], r[1], r[2]) not in have]


def append(path, rows):
    text = open(path).read() if os.path.exists(path) else ""
    fresh = new_rows(rows, text)
    with open(path, "a") as f:
        if not text:
            f.write("\t".join(COLUMNS) + "\n")
        elif not text.endswith("\n"):
            f.write("\n")
        for r in fresh:
            f.write("\t".join(r) + "\n")
    return fresh


# ---- running (not covered by the unit tests: they read fixtures) ----

def warn(msg):
    print("maturity-ledger: " + msg, file=sys.stderr)


def run(cmd, env=None, cwd=ROOT):
    return subprocess.run(["nice", "-n", "10"] + cmd, cwd=cwd, env=env, capture_output=True, text=True)


def git(*args):
    return subprocess.run(["git"] + list(args), cwd=ROOT, capture_output=True, text=True).stdout.strip()


def rust_extractor(env):
    """RIVUS_RUST if set; else the extractor of this tree, built in extractor/rust/target (not the shared install)."""
    if env.get("RIVUS_RUST"):
        return env["RIVUS_RUST"]
    if not shutil.which("cargo"):
        return None
    d = os.path.join(ROOT, "extractor", "rust")
    r = run(["cargo", "build"], env=dict(env, CARGO_TARGET_DIR=os.path.join(d, "target")), cwd=d)
    if r.returncode != 0:
        tail = r.stderr.strip().splitlines()
        warn("rust: the extractor did not build" + (": " + tail[-1] if tail else ""))
        return None
    return os.path.join(d, "target", "debug", "rivus-rust")


def collect_zoo(env, values):
    if shutil.which(env.get("RIVUS_DART", "dart")):
        env.setdefault("RIVUS_DART", "dart")
    else:
        warn("dart: no `dart` command; the Dart zoo is skipped")
    rust = rust_extractor(env)
    if rust:
        env["RIVUS_RUST"] = rust
    else:
        warn("rust: no extractor (set RIVUS_RUST, or install cargo); the Rust zoo is skipped")
    r = run(["go", "test", "-p", "2", "./cmd/rivus", "-run", r"TestZoo(Go|Dart|Rust)$", "-v", "-count=1", "-timeout", "30m"], env=env)
    out = r.stdout + r.stderr
    if r.returncode != 0:
        warn("the zoo tests failed (exit %d); the numbers below are what the lines gave" % r.returncode)
    todo, miss = parse_todo(out), parse_miss(out)
    for lang in LANGS:
        if lang in todo:
            values[lang]["todo"] = todo[lang]
        else:
            warn("%s: no 'parity marks' line (the test did not run?): todo left empty" % lang)
        if lang in miss:
            values[lang]["miss"] = miss[lang]
        else:
            warn("%s: no '@miss' line: miss left empty" % lang)


def collect_go_edges(env, values):
    r = run(["go", "run", "./bench/observed-edges", "run", "-zoo", "bench/zoo-go"], env=env)
    n = parse_no_edge_go(r.stdout)
    if n is None:
        lines = (r.stdout + r.stderr).strip().splitlines()
        why = next((l for l in lines if l.startswith("observed-edges:")), lines[-1] if lines else "no output")
        warn("go: observed-edges gave no no_edge line (exit %d: %s); no_edge left empty" % (r.returncode, why))
    else:
        values["go"]["no_edge"] = n


def collect_dart_edges(env, values):
    """README of bench/zoo-dart/observed, method A, in a temporary directory."""
    dart = shutil.which(env.get("RIVUS_DART", "dart"))
    if not dart:
        warn("dart: no `dart` command; no_edge left empty")
        return
    with tempfile.TemporaryDirectory(prefix="maturity-dart-") as T:
        steps = [
            (["go", "build", "-o", T + "/rivus", "./cmd/rivus"], ROOT, {}),
            (["go", "test", "./cmd/rivus", "-run", "^TestZooDartGenerate$", "-count=1", "-update"], ROOT, {}),
            ([dart, "pub", "get"], ROOT + "/extractor/dart", {}),
            ([dart, "pub", "get"], ROOT + "/bench/zoo-dart/cells", {}),
            ([T + "/rivus", "index", "-C", ".", "-graph", T + "/zoo-dart.db", "./..."], ROOT + "/bench/zoo-dart/cells",
             {"RIVUS_DART_EXTRACTOR": "dart run " + ROOT + "/extractor/dart/bin/rivus_dart.dart"}),
            ([dart, "pub", "get"], ROOT + "/bench/zoo-dart/exec", {}),
            ([dart, "test", "test/zoo_test.dart"], ROOT + "/bench/zoo-dart/exec", {"ZOO_STACKS": "1", "ZOO_STACKS_OUT": T + "/stacks.json"}),
        ]
        for cmd, cwd, extra in steps:
            r = run(cmd, env=dict(env, **extra), cwd=cwd)
            if r.returncode != 0:
                tail = (r.stdout + r.stderr).strip().splitlines()
                warn("dart: `%s` failed (exit %d: %s); no_edge left empty" % (" ".join(cmd[:3]), r.returncode, tail[-1] if tail else ""))
                return
        r = run([sys.executable, ROOT + "/bench/zoo-dart/observed/compare.py", T + "/stacks.json", T + "/zoo-dart.db"], env=env)
        n = parse_no_edge_dart(r.stdout)
        if r.returncode != 0 or n is None:
            warn("dart: compare.py gave no summary (exit %d); no_edge left empty" % r.returncode)
        else:
            values["dart"]["no_edge"] = n


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--dry-run", action="store_true", help="print the rows, add nothing")
    ap.add_argument("--no-edges", action="store_true", help="leave no_edge empty (the zoo tests only)")
    ap.add_argument("--ledger", default=LEDGER)
    a = ap.parse_args(argv)
    env = dict(os.environ)
    env.setdefault("GOMAXPROCS", "6")
    if git("status", "--porcelain", "--untracked-files=no"):
        warn("the tree has changes: the commit in the row is not what was measured")
    commit = git("rev-parse", "--short=8", "HEAD")
    date = datetime.date.today().isoformat()
    values = {l: {} for l in LANGS}
    collect_zoo(env, values)
    if not a.no_edges:
        collect_go_edges(env, values)
        collect_dart_edges(env, values)
    rows = make_rows(date, commit, values)
    text = open(a.ledger).read() if os.path.exists(a.ledger) else ""
    fresh = new_rows(rows, text)
    for r in rows:
        print("\t".join(r) + ("" if r in fresh else "\t(already in the ledger)"))
    if not a.dry_run:
        append(a.ledger, rows)
    return 0


if __name__ == "__main__":
    sys.exit(main())
