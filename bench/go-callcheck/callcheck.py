#!/usr/bin/env python3
"""The check of call positions for Go (B(3) of docs/ROADMAP.md): the calls the source writes, against the `call` and
`unsupported` instructions of the IR that the Go extractor (`rivus debug extract`) wrote for it. The Dart tool is
bench/dart-callcheck/callcheck.py; the two have the same numbers and the same rules.

  python3 bench/go-callcheck/callcheck.py [--json] [--tree DIR] DIR         (DIR: a Go module; ./... without tests)
  python3 bench/go-callcheck/callcheck.py --syntax S.jsonl --ir X.jsonl     (the two inputs already made)

What it answers: does the extractor leave out a call that the source writes, and say nothing? It does not say that the
calls it found are right, and it does not look at the calls that the IR has without an argument list in the source
(the implicit calls: `rt.rundefers`, the call of a range-over-func body, the synthetic `init`, the wrappers of method values).

THE DEFINITION OF A POSITION (one for both sides; if they disagree, this definition is what is changed):
  a call is the offset (0-based, in bytes, file relative to DIR) of the "(" that opens its argument list
  (go/ast CallExpr.Lparen). The syntax side (bench/go-callcheck/syntax) takes it from the syntax tree; go/ssa gives an
  ordinary call the same token and the extractor writes it as the Pos of the `call` instruction (internal/frontend/goir/goir.go
  `c.pos(in.Pos())`). The call that a `go` or a `defer` statement makes has the position of the keyword (GoStmt.Go,
  DeferStmt.Defer) on both sides: go/ssa gives that position, not the "(". Conversions and constants (len of an array,
  unsafe.Sizeof) are not calls (go/types). A call is "found" when the IR has a `call` instruction at (file, offset). It is
  "reported" when the IR has an `unsupported` instruction at that offset or inside the callee part of the call (the
  start of the call .. the offset); an `unsupported` is bound to the innermost such call.
  Four numbers:
    call_mismatch    the number of functions (the outermost declaration that holds the call; a closure is part of it) that
                     have at least one call that is neither found nor reported (and not allowed below).
    call_unsupported the number of calls that are only reported by an `unsupported` (not a mismatch: said, not dropped).
    call_dead        the number of calls that the IR does not have because go/ssa does not keep them, and for a reason that
                     the source states (DEAD below: not reachable by go/cfg; in a function literal assigned to `_`). A call
                     that the IR has is found, whatever the source says of it.
    allowed          the calls of the forms that go/ssa does not make a call (ALLOW below), by name.
  The tool only reads; 0 is not "no problem" (see bench/README.md, "The ledger of maturity").
"""
import argparse, collections, json, os, subprocess, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# The differences that are not a drop, by name: (predicate over a syntax record, the reason). An entry needs a reason;
# nothing is allowed without one (test_go_callcheck.py).
ALLOW = {
    "builtin_panic": (lambda s: s["kind"] == "builtin" and s["name"] == "panic",
                      "go/ssa has panic as the instruction ssa.Panic, the end of a block (the IR: a throw terminator), not as a call"),
    "builtin_make": (lambda s: s["kind"] == "builtin" and s["name"] == "make",
                     "go/ssa has make as an allocation (ssa.MakeSlice, MakeMap, MakeChan; the IR: alloc), not as a call"),
    "builtin_new": (lambda s: s["kind"] == "builtin" and s["name"] == "new",
                    "go/ssa has new as an allocation (ssa.Alloc; the IR: alloc), not as a call"),
    "builtin_min_max": (lambda s: s["kind"] == "builtin" and s["name"] in ("min", "max"),
                        "go/ssa lowers min and max to comparisons and selects, not to a call"),
}

# The reasons that make a call dead (the "dead" field of a syntax record). A value that is not here is not a reason.
DEAD = {
    "unreachable": "go/cfg: the statement is in a block that the entry of its function cannot reach; go/ssa deletes unreachable blocks",
    "discarded_closure": "the call is in a function literal whose value is assigned to the blank identifier, so nothing can call it; go/ssa does not keep its body",
}


def parse_syntax(text):
    return [json.loads(l) for l in text.splitlines() if l.strip()]


def parse_ir(text):
    """(the set of (file, offset) of the `call` instructions, the list of (file, offset) of the `unsupported`, the set of files
    that have an instruction with a position). An instruction whose position has no file is in the file of its function."""
    calls, uns, files = set(), [], set()
    for line in text.splitlines():
        if not line.startswith("{"):
            continue
        o = json.loads(line)
        if o.get("kind") != "func":
            continue
        fn = o["func"]
        for b in fn.get("blocks") or []:
            for i in b.get("instrs") or []:
                pos = i.get("pos")
                if not pos:
                    continue
                f = pos.get("file") or fn.get("file", "")
                files.add(f)
                key = (f, pos.get("offset", 0))
                if i.get("op") == "call":
                    calls.add(key)
                elif i.get("op") == "unsupported":
                    uns.append(key)
    return calls, uns, files


def compare(syntax, calls, uns, files):
    by_file = collections.defaultdict(list)
    for s in syntax:
        by_file[s["file"]].append(s)
    reported = set()  # (file, offset) of the calls that an `unsupported` reports
    for f, off in uns:
        best = None
        for s in by_file.get(f, ()):
            if s["start"] <= off <= s["offset"] and (f, s["offset"]) not in calls:
                if best is None or (s["start"], -s["offset"]) > (best["start"], -best["offset"]):
                    best = s
        if best:
            reported.add((f, best["offset"]))
    mismatches, allowed, dead, found, n_reported = [], collections.Counter(), collections.Counter(), 0, 0
    for s in syntax:
        key = (s["file"], s["offset"])
        if key in calls:
            found += 1
        elif key in reported:
            n_reported += 1
        else:
            name = next((n for n, (pred, _) in ALLOW.items() if pred(s)), None)
            if name:
                allowed[name] += 1
            elif s.get("dead") in DEAD:
                dead[s["dead"]] += 1
            else:
                mismatches.append(s)
    funcs = {(m["file"], m["owner_line"], m["owner"]) for m in mismatches}
    return {
        "syntax_calls": len(syntax),
        "found": found,
        "call_unsupported": n_reported,
        "call_dead": sum(dead.values()),
        "dead": dict(dead),
        "call_mismatch": len(funcs),
        "mismatches": mismatches,
        "allowed": dict(allowed),
        "files_without_ir": sorted(f for f in by_file if f not in files),
    }


def run(cmd, cwd):
    r = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("callcheck: `%s` failed: %s" % (" ".join(cmd[:3]), r.stderr.strip()[-400:]))
    return r.stdout


def run_syntax(directory):
    return run(["go", "run", "./bench/go-callcheck/syntax", os.path.abspath(directory)], ROOT)


def run_extractor(directory, tree):
    """The IR of `rivus debug extract` of the tree (the tree of this checkout, or a copy of it for the check of the check)."""
    with tempfile.TemporaryDirectory(prefix="go-callcheck-") as t:
        exe = os.path.join(t, "rivus")
        run(["go", "build", "-o", exe, "./cmd/rivus"], tree)
        return run([exe, "debug", "extract", "-C", os.path.abspath(directory), "./..."], ROOT)


def report(r, out=sys.stdout):
    print("syntax_calls %d" % r["syntax_calls"], file=out)
    print("found %d" % r["found"], file=out)
    print("call_unsupported %d" % r["call_unsupported"], file=out)
    print("call_dead %d" % r["call_dead"], file=out)
    print("dead %s" % (json.dumps(r["dead"], sort_keys=True) if r["dead"] else "none"), file=out)
    print("allowed %s" % (json.dumps(r["allowed"], sort_keys=True) if r["allowed"] else "none"), file=out)
    print("call_mismatch %d" % r["call_mismatch"], file=out)
    for f in r["files_without_ir"]:
        print("file_without_ir %s" % f, file=out)
    for m in r["mismatches"]:
        print("mismatch %s:%d %s in `%s`" % (m["file"], m["line"], m["kind"] + (" " + m["name"] if m.get("name") else ""), m["owner"]), file=out)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("dir", nargs="?")
    ap.add_argument("--syntax", help="the output of bench/go-callcheck/syntax (instead of running it)")
    ap.add_argument("--ir", help="the IR of the extractor (instead of running it)")
    ap.add_argument("--tree", default=ROOT, help="the tree whose cmd/rivus is built and run (a copy, for the check of the check)")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args(argv)
    if not a.dir and not (a.syntax and a.ir):
        ap.error("a directory, or both --syntax and --ir")
    syntax = parse_syntax(open(a.syntax).read() if a.syntax else run_syntax(a.dir))
    calls, uns, files = parse_ir(open(a.ir).read() if a.ir else run_extractor(a.dir, a.tree))
    r = compare(syntax, calls, uns, files)
    if a.json:
        json.dump(r, sys.stdout, indent=1)
        print()
    else:
        report(r)
    return 0


if __name__ == "__main__":
    sys.exit(main())
