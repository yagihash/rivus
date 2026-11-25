#!/usr/bin/env python3
"""The check of call positions for Dart (B(3) of docs/ROADMAP.md): the calls the source writes, against the `call` and
`unsupported` instructions of the IR that the Dart extractor wrote for it.

  python3 bench/dart-callcheck/callcheck.py [--json] [--extractor DIR] DIR       (DIR: a directory with Dart packages, `dart pub get` done)
  python3 bench/dart-callcheck/callcheck.py --syntax S.jsonl --ir X.jsonl        (the two inputs already made)

What it answers: does the extractor leave out a call that the source writes, and say nothing? It does not say that the
calls it found are right, and it does not look at the calls that the IR has without an argument list in the source.

THE DEFINITION OF A POSITION (one for both sides; if they disagree, this definition is what is changed):
  a call is the offset (0-based, UTF-16 units of the file, relative path from DIR) of the "(" that opens its argument list.
  The syntax side (bin/syntax_calls.dart) takes it from the argument list of the call node; the extractor's Pos of a
  `call` instruction is the same token (extractor/dart/lib/program.dart `callPos`: args.leftParenthesis.offset;
  extractor/dart/lib/exprs.dart `f.pos(e, e.argumentList.leftParenthesis.offset)`). A call is "found" when the IR has a
  `call` instruction at (file, offset). It is "reported" when the IR has an `unsupported` instruction at that offset or
  inside the callee part of the call (the beginning of the node .. the "("; the extractor writes the `unsupported` of
  a `super(..)` initializer at its first token); an `unsupported` is bound to the innermost such call.
  Two numbers:
    call_mismatch    the number of functions (the outermost member that holds the call: a function, a method, a
                     constructor, a field, a top-level variable, an enum constant; a closure is part of its member) that have
                     at least one call that is neither found nor reported (and not allowed below).
    call_unsupported the number of calls that are only reported by an `unsupported` (not a mismatch: said, not dropped).
  The tool only reads; 0 is not "no problem" (see bench/README.md, "The ledger of maturity").
"""
import argparse, collections, json, os, subprocess, sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
HERE = os.path.dirname(os.path.abspath(__file__))

# The differences that are not a drop, by name: (predicate over a syntax record, the reason). An entry needs a reason;
# nothing is allowed without one (test_dart_callcheck.py). A cascade needs no entry: its sections are counted one by one,
# each at its own "(", and the extractor makes one call for each of them.
ALLOW = {}


def parse_syntax(text):
    return [json.loads(l) for l in text.splitlines() if l.strip()]


def parse_ir(text):
    """(the set of (file, offset) of the `call` instructions, the list of (file, offset) of the `unsupported`, the set of files
    that have an instruction with a position)."""
    calls, uns, files = set(), [], set()
    for line in text.splitlines():
        if not line.startswith("{"):
            continue
        o = json.loads(line)
        if o.get("kind") != "func":
            continue
        for b in o["func"].get("blocks") or []:
            for i in b.get("instrs") or []:
                pos = i.get("pos")
                if not pos:
                    continue
                files.add(pos.get("file", ""))
                key = (pos.get("file", ""), pos.get("offset", 0))
                if i.get("op") == "call":
                    calls.add(key)
                elif i.get("op") == "unsupported":
                    uns.append(key)
    return calls, uns, files


def compare(syntax, calls, uns, files):
    by_file = collections.defaultdict(list)
    for s in syntax:
        by_file[s["file"]].append(s)
    reported = set()  # (file, offset of the "(") of the calls that an `unsupported` reports
    for f, off in uns:
        best = None
        for s in by_file.get(f, ()):
            if s["start"] <= off <= s["offset"] and (f, s["offset"]) not in calls:
                if best is None or (s["start"], -s["offset"]) > (best["start"], -best["offset"]):
                    best = s
        if best:
            reported.add((f, best["offset"]))
    mismatches, allowed, found, n_reported = [], collections.Counter(), 0, 0
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
            else:
                mismatches.append(s)
    funcs = {(m["file"], m["owner_line"], m["owner"]) for m in mismatches}
    return {
        "syntax_calls": len(syntax),
        "found": found,
        "call_unsupported": n_reported,
        "call_mismatch": len(funcs),
        "mismatches": mismatches,
        "allowed": dict(allowed),
        "files_without_ir": sorted(f for f in by_file if f not in files),
    }


def run_syntax(directory):
    env = dict(os.environ)
    r = subprocess.run([env.get("RIVUS_DART", "dart"), "run", "bin/syntax_calls.dart", os.path.abspath(directory)], cwd=HERE, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("callcheck: the syntax side failed: " + r.stderr.strip()[-400:])
    return r.stdout


def run_extractor(directory, extractor):
    env = dict(os.environ)
    r = subprocess.run([env.get("RIVUS_DART", "dart"), "run", os.path.join(extractor, "bin", "rivus_dart.dart"), os.path.abspath(directory)], capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("callcheck: the extractor failed: " + r.stderr.strip()[-400:])
    return r.stdout


def report(r, out=sys.stdout):
    print("syntax_calls %d" % r["syntax_calls"], file=out)
    print("found %d" % r["found"], file=out)
    print("call_unsupported %d" % r["call_unsupported"], file=out)
    print("allowed %s" % (json.dumps(r["allowed"], sort_keys=True) if r["allowed"] else "none"), file=out)
    print("call_mismatch %d" % r["call_mismatch"], file=out)
    for f in r["files_without_ir"]:
        print("file_without_ir %s" % f, file=out)
    for m in r["mismatches"]:
        print("mismatch %s:%d %s in `%s`" % (m["file"], m["line"], m["kind"], m["owner"]), file=out)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("dir", nargs="?")
    ap.add_argument("--syntax", help="the output of bin/syntax_calls.dart (instead of running it)")
    ap.add_argument("--ir", help="the IR of the extractor (instead of running it)")
    ap.add_argument("--extractor", default=os.path.join(ROOT, "extractor", "dart"), help="the directory of the extractor to run (a copy, for the check of the check)")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args(argv)
    if not a.dir and not (a.syntax and a.ir):
        ap.error("a directory, or both --syntax and --ir")
    syntax = parse_syntax(open(a.syntax).read() if a.syntax else run_syntax(a.dir))
    calls, uns, files = parse_ir(open(a.ir).read() if a.ir else run_extractor(a.dir, a.extractor))
    r = compare(syntax, calls, uns, files)
    if a.json:
        json.dump(r, sys.stdout, indent=1)
        print()
    else:
        report(r)
    return 0


if __name__ == "__main__":
    sys.exit(main())
