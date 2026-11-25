#!/usr/bin/env python3
"""The ledger of maturity: four numbers per language, one row each, added to bench/maturity-ledger.tsv.

  python3 bench/maturity_ledger.py [--dry-run] [--no-edges] [--ledger FILE]      (make maturity-ledger)

todo, miss: the zoo test of each language; todo_marked: the @todo(F/..: PARITY: ..) marks in the sources (na.txt, lib.rs, the Go zoo) (`go test ./cmd/rivus -run 'TestZoo(Go|Dart|Rust)$' -v`).
no_edge: Go: `go run ./bench/observed-edges run -zoo bench/zoo-go`; Dart: bench/zoo-dart/observed/compare.py (needs `dart`).
call_mismatch, call_unsupported: Dart only: bench/dart-callcheck/callcheck.py on bench/zoo-dart/cells (the functions with a call that the source writes
and the IR has neither as a `call` nor as an `unsupported`; the calls that only an `unsupported` reports). Go and Rust: empty (no such check yet).
An input that cannot be had leaves its column empty, and the reason goes to standard error: an empty cell is not a zero.
What a number does not show is in bench/README.md (the section "The ledger of maturity").
The script only adds rows. The commit is made by a person. Standard library only.
"""
import argparse, datetime, os, re, shutil, subprocess, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LEDGER = os.path.join(ROOT, "bench", "maturity-ledger.tsv")
COLUMNS = ["date", "rivus_commit", "lang", "cells", "na", "todo", "todo_marked", "miss", "no_edge", "calls_observed", "call_mismatch", "call_unsupported"]
LANGS = ["go", "dart", "rust"]

MARKS_RE = re.compile(r"zoo-(go|dart|rust): parity marks: @na (\d+) \[([^\]]*)\] \| @todo (\d+) \[([^\]]*)\]")
MISS_RE = re.compile(r"zoo-(go|dart|rust): cells \d+ / cases \d+ \| found \d+ / \d+ \| @miss (\d+) \(trace \d+, silent \d+, no rule \d+\)")
# the same expression as zooParityTodoRe in cmd/rivus/zoo_test.go: a mark names a cell (F/<family>/<shape>), so the
# explanation "@todo(F/<family>/<shape>: PARITY: reason)" in bench/zoo-dart/na.txt is not a mark
TODO_MARK_RE = re.compile(r"@todo\((F/[a-z0-9/-]+):\s*PARITY:\s*([^)]+)\)")
MARK_FILES = {"go": ["bench/zoo-go"], "dart": ["bench/zoo-dart/na.txt"], "rust": ["bench/zoo-rust/cells/src/lib.rs"]}
CELLS_RE = re.compile(r"zoo-(go|dart|rust): cells (\d+) / cases \d+ \| found")
GO_COMPARED_RE = re.compile(r"^\s*(ok|no_edge|no_callsite|id_unmapped)\s+(\d+)\s*$", re.M)
DART_CALL_RE = re.compile(r"^call\s+\w+\s+(\d+)\s*$", re.M)
NO_EDGE_RE = re.compile(r"^\s*no_edge\s+(\d+)\s*$", re.M)
COMPARE_RE = re.compile(r"^\w+\s+no_edge\s+(\d+)\s*$", re.M)
CALL_MISMATCH_RE = re.compile(r"^call_mismatch\s+(\d+)\s*$", re.M)
CALL_UNSUPPORTED_RE = re.compile(r"^call_unsupported\s+(\d+)\s*$", re.M)


def parse_todo(text):
    """{lang: @todo count} from the "parity marks" lines of the zoo tests."""
    return {m.group(1): int(m.group(4)) for m in MARKS_RE.finditer(text)}


def count_todo_marked(text):
    """The cells that the text declares with @todo(F/..: PARITY: ..) (a name once), not the ones that have no mark."""
    return len({m.group(1) for m in TODO_MARK_RE.finditer(text)})


def parse_na(text):
    """{lang: @na count} from the same "parity marks" lines."""
    return {m.group(1): int(m.group(2)) for m in MARKS_RE.finditer(text)}


def parse_cells(text):
    """{lang: the number of cells of the zoo} from the "cells N / cases M | found" lines."""
    return {m.group(1): int(m.group(2)) for m in CELLS_RE.finditer(text)}


def parse_calls_go(text):
    """The calls that observed-edges compared (ok + no_edge + no_callsite + id_unmapped; `outside` is not in it), or None."""
    ms = GO_COMPARED_RE.findall(text)
    return sum(int(n) for _, n in ms) if ms else None


def parse_calls_dart(text):
    """The calls that compare.py compared (the lines "call <class> N"; the "sdk" lines are callbacks, not calls), or None."""
    ms = DART_CALL_RE.findall(text)
    return sum(int(x) for x in ms) if ms else None


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


def parse_call_check(text):
    """(call_mismatch, call_unsupported) from the summary of bench/dart-callcheck/callcheck.py; None for a number it does not have."""
    a, b = CALL_MISMATCH_RE.search(text), CALL_UNSUPPORTED_RE.search(text)
    return (int(a.group(1)) if a else None, int(b.group(1)) if b else None)


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
    if text and text.splitlines()[0] != "\t".join(COLUMNS):
        raise SystemExit("maturity-ledger: the header of %s is not the columns of this script" % path)
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
    todo, miss, na, cells = parse_todo(out), parse_miss(out), parse_na(out), parse_cells(out)
    for lang in LANGS:
        for col, d in (("cells", cells), ("na", na)):
            if lang in d:
                values[lang][col] = d[lang]
            else:
                warn("%s: no line for %s: left empty" % (lang, col))
        if lang in todo:
            values[lang]["todo"] = todo[lang]
        else:
            warn("%s: no 'parity marks' line (the test did not run?): todo left empty" % lang)
        if lang in miss:
            values[lang]["miss"] = miss[lang]
        else:
            warn("%s: no '@miss' line: miss left empty" % lang)


def collect_marked(values):
    for lang, paths in MARK_FILES.items():
        names, seen = 0, False
        for rel in paths:
            full = os.path.join(ROOT, rel)
            files = [full] if os.path.isfile(full) else [os.path.join(d, f) for d, _, fs in os.walk(full) for f in fs if f.endswith((".go", ".txt"))]
            for f in files:
                seen = True
                names += count_todo_marked(open(f, errors="replace").read())
        if seen:
            values[lang]["todo_marked"] = names
        else:
            warn("%s: no file to read the @todo marks from: todo_marked left empty" % lang)


def collect_go_edges(env, values):
    r = run(["go", "run", "./bench/observed-edges", "run", "-zoo", "bench/zoo-go"], env=env)
    n = parse_no_edge_go(r.stdout)
    if n is None:
        lines = (r.stdout + r.stderr).strip().splitlines()
        why = next((l for l in lines if l.startswith("observed-edges:")), lines[-1] if lines else "no output")
        warn("go: observed-edges gave no no_edge line (exit %d: %s); no_edge left empty" % (r.returncode, why))
    else:
        values["go"]["no_edge"] = n
        values["go"]["calls_observed"] = parse_calls_go(r.stdout)


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
            values["dart"]["calls_observed"] = parse_calls_dart(r.stdout)


def collect_call_check(env, values):
    """bench/dart-callcheck/callcheck.py on bench/zoo-dart/cells (the generated files of the cells, and `dart pub get`, as for no_edge)."""
    dart = shutil.which(env.get("RIVUS_DART", "dart"))
    if not dart:
        warn("dart: no `dart` command; call_mismatch and call_unsupported left empty")
        return
    steps = [
        (["go", "test", "./cmd/rivus", "-run", "^TestZooDartGenerate$", "-count=1", "-update"], ROOT),
        ([dart, "pub", "get"], ROOT + "/extractor/dart"),
        ([dart, "pub", "get"], ROOT + "/bench/zoo-dart/cells"),
        ([dart, "pub", "get"], ROOT + "/bench/dart-callcheck"),
        ([sys.executable, ROOT + "/bench/dart-callcheck/callcheck.py", ROOT + "/bench/zoo-dart/cells"], ROOT),
    ]
    for cmd, cwd in steps:
        r = run(cmd, env=env, cwd=cwd)
        if r.returncode != 0:
            tail = (r.stdout + r.stderr).strip().splitlines()
            warn("dart: `%s` failed (exit %d: %s); call_mismatch and call_unsupported left empty" % (" ".join(cmd[:3]), r.returncode, tail[-1] if tail else ""))
            return
    mismatch, unsupported = parse_call_check(r.stdout)
    if mismatch is None:
        warn("dart: callcheck.py gave no summary; call_mismatch and call_unsupported left empty")
        return
    values["dart"]["call_mismatch"], values["dart"]["call_unsupported"] = mismatch, unsupported


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--dry-run", action="store_true", help="print the rows, add nothing")
    ap.add_argument("--no-edges", action="store_true", help="leave no_edge and the call check empty (the zoo tests only)")
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
    collect_marked(values)
    if not a.no_edges:
        collect_go_edges(env, values)
        collect_dart_edges(env, values)
        collect_call_check(env, values)
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
