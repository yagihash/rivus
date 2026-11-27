#!/usr/bin/env python3
"""The check of call positions for Rust (B(3) of docs/ROADMAP.md): the calls the source writes, against the `call` and
`unsupported` instructions of the IR that the Rust extractor wrote for it. The Go and Dart tools are bench/go-callcheck and
bench/dart-callcheck; the question and the numbers are the same.

  python3 bench/rust-callcheck/callcheck.py [--json] [--only PREFIX]... --ir X.jsonl DIR        (DIR: the workspace that the IR was made from)
  python3 bench/rust-callcheck/callcheck.py --syntax S.jsonl --ir X.jsonl    (the two inputs already made)

The IR is not made here: this tool never runs the extractor. `RIVUS_SAVE_IR=<dir> RIVUS_RUST=<binary> go test ./cmd/rivus -run
TestSaveRustIRForCallcheck` writes `curated.ir.jsonl` (DIR: bench/rust-curated) and `zoo-rust.ir.jsonl` (DIR: bench/zoo-rust).
The syntax side (bench/rust-callcheck/syntax, on syn) reads the sources only: nothing is built or run in DIR.

What it answers: does the extractor leave out a call that the source writes, and say nothing? It does not say that the
calls it found are right (the callee, the arguments), and it does not look at the calls that the IR has without an argument
list in the source (`?`, an operator, an auto-deref, `for`, `.await`, what a macro or a derive makes).

THE DEFINITION OF A POSITION (one for both sides; if they disagree, this definition is what is changed):
  a call is the start of its expression (`f(x)` starts at `f`, `a.b(x)` at `a`): file (relative to DIR), line and column (1-based,
  in characters), and the name that it calls (the method, or the last segment of the path of `f(x)`). This is not the "(" of
  Go and Dart: the MIR that the extractor reads has the span of the whole call expression and not the `fn_span` (rustc_public
  drops it), so the IR has no "(" (the extractor's Pos of a `call` is extractor/rust/src/lower.rs `pos`, the start of the
  span of the Call terminator). The start is shared by the calls of a chain (`a.f().g()`: both start at `a`), so the key is
  the start and the name; the calls that this does not tell apart are compared by their number at the position (`weak`).
  A call is "found" when the IR has a `call` at the position whose name is the name of the call (the name of the callee of the
  IR is the last segment of its symbol). A call of a function value (`f(x)` with `f` a closure, a function pointer or a
  `dyn Fn`) is found by a call of `Fn::call`, `FnMut::call_mut`, `FnOnce::call_once`, or of a callee of the kind `value` at the
  position: the IR does not have the name of the variable. It is "reported" when the IR has an `unsupported` instruction inside
  the call (from its start to its end); an `unsupported` is bound to the innermost such call.
  A call of a function value is a call (`value_callee` of the syntax record: the callee is a local variable or not a path) that the IR
  has under another name: it is found by a call of Fn (or of the kind `value`) at the position, counted in `found_fn_value`, or else by
  any call of the position that is not an implicit one (IMPLICIT), counted in `found_loose`.
  Numbers:
    found_fn_value, found_loose   the calls found by those rules (a part of `found`; a drop of one of them is seen only if no
                     matching call is left at the position; `found_loose` is the weakest).
    call_mismatch    the number of functions (the outermost fn, method or closure-holding item that has the call: `owner` and its
                     line) that have at least one call that is neither found nor reported (and not allowed below).
    call_unsupported the number of calls that are only reported by an `unsupported` (not a mismatch: said, not dropped).
    weak             the number of counted calls that share their position AND name with another counted call (so a drop of one of
                     them is seen only if the number of calls at the position falls), a part of `found`; and `shared_start`, those
                     that share the start only (the name tells them apart).
    excluded         the calls that are not counted, by kind (EXCLUDED below). Not a mismatch, not hidden: a number for each kind.
    macro_opaque     the macros that the syntax side cannot look into: the arguments are not a list of expressions, and
                     `macro_rules!` definitions. The calls in them are not seen at all (a drop there is not found).
  The tool only reads; 0 is not "no problem" (see bench/README.md, "The ledger of maturity").
"""
import argparse, collections, json, os, re, subprocess, sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
HERE = os.path.dirname(os.path.abspath(__file__))

# The differences that are not a drop, by name: (predicate over a syntax record, the reason). An entry needs a reason;
# nothing is allowed without one (test_rust_callcheck.py).
ALLOW = {
    "slice_len": (lambda s: s["kind"] == "method" and s["name"] == "len",
                  "`<[T]>::len` is lowered by rustc (LowerSliceLenCalls) to the length of the place, an operation of MIR and not a Call; the syntax cannot tell a slice from a Vec (whose `len` is a call), so a `len` without a call in the IR is allowed, by name, and counted as `allowed`"),
}

# The calls that the syntax side does not count, by kind (the `excl` of a syntax record), and why.
EXCLUDED = {
    "ctor": "`Some(x)`, `Ok(x)`, `P(x)`: a tuple struct or a variant is an aggregate in MIR, not a call; syn does not resolve names, so a path whose last segment starts with an upper-case letter is read as one",
    "cfg": "under another #[cfg(..)]: the syntax does not evaluate the condition (a feature that is off is not compiled), so the code is not counted, whichever way the build went",
    "cfg_test": "under #[cfg(test)] or #[test]: the extractor does not compile it (no --include-tests)",
    "const_init": "the initializer of a const or a static: evaluated by the compiler, it is not in the MIR of a function",
    "unreachable": "after a return, break, continue or panic-like macro of the same block, or in `if false`: MIR drops it (-Zmir-opt-level=1); the same as call_dead of Go, stated by the syntax",
}

FN_FAMILY = {"call", "call_mut", "call_once"}
# The names of the calls that the IR has for what is not written as a call (`?`, operators, auto-deref, `for`, `.await`, drop):
# the call of a function value is not found by one of them.
IMPLICIT = {"branch", "from_residual", "deref", "deref_mut", "index", "index_mut", "into_iter", "next", "into_future", "new_unchecked", "drop",
            "add", "sub", "mul", "div", "rem", "neg", "not", "eq", "ne", "lt", "le", "gt", "ge", "partial_cmp", "add_assign", "sub_assign", "new_display"}


def parse_syntax(text):
    return [json.loads(l) for l in text.splitlines() if l.strip()]


def callee_name(symbol):
    """The name of a callee: the last segment of its symbol (`<W<T> as Tr>::m` is m, `f::{closure#0}` is {closure})."""
    if not symbol:
        return ""
    last = symbol.rsplit("::", 1)[-1]
    return "{closure}" if last.startswith("{closure") else last


def parse_ir(text):
    """(the dict (file, line, col) -> list of (name, kind) of the `call` instructions, the list of (file, line, col) of the
    `unsupported`, the set of files that have an instruction with a position). A position without a file is in the file of its
    function (the extractor writes the file once, for the function)."""
    calls, uns, files = collections.defaultdict(list), [], set()
    for line in text.splitlines():
        if not line.startswith("{"):
            continue
        o = json.loads(line)
        fn = o.get("func")
        if o.get("kind") != "func" or not isinstance(fn, dict):
            continue
        for b in fn.get("blocks") or []:
            for i in b.get("instrs") or []:
                pos = i.get("pos")
                if not pos or "line" not in pos:
                    continue
                f = pos.get("file") or fn.get("file", "")
                files.add(f)
                key = (f, pos["line"], pos.get("col", 0))
                if i.get("op") == "call":
                    c = i.get("callee") or {}
                    calls[key].append((callee_name(c.get("symbol") or c.get("func")), c.get("kind", "")))
                elif i.get("op") == "unsupported":
                    uns.append(key)
    return calls, uns, files


def counted(syntax):
    """The calls that are counted, and the number of the others by kind; and the macros that cannot be looked into."""
    calls, excl, opaque = [], collections.Counter(), collections.Counter()
    for s in syntax:
        k = s.get("kind")
        if k in ("call", "method"):
            if s.get("excl"):
                excl[s["excl"]] += 1
            else:
                calls.append(s)
        elif k == "macro" and not s.get("args_parse"):
            opaque["macro_rules" if s.get("excl") == "macro_rules" else "arguments_not_expressions"] += 1
    return calls, dict(excl), dict(opaque)


def compare(syntax, ir_calls, uns, files):
    calls, excl, opaque = counted(syntax)
    by_file = collections.defaultdict(list)
    for s in syntax:
        by_file[s["file"]].append(s)
    pos_of = lambda s: (s["file"], s["line"], s["col"])
    # the calls at one start, in the order of the source
    group = collections.defaultdict(list)
    for s in calls:
        group[pos_of(s)].append(s)
    found_ids, loose, fnval, found = set(), set(), set(), 0
    for key, ss in group.items():
        pool = list(ir_calls.get(key, ()))
        pending = []
        for s in ss:
            hit = next((j for j, (n, _) in enumerate(pool) if n == s["name"] and n != ""), None)
            if hit is None:
                pending.append(s)
            else:
                pool.pop(hit)
                found_ids.add(id(s))
        for s in pending:
            # a call of a function value (a local variable, `(x.f)(..)`): the IR has the call of Fn, or of the function that the
            # value is known to be (the compiler propagates `let f = g; f(x)` into a call of `g`), not the name of the variable
            if not s.get("value_callee"):
                continue
            hit = next((j for j, (n, k) in enumerate(pool) if k == "value" or n in FN_FAMILY), None)
            via_fn = hit is not None
            if hit is None:
                hit = next((j for j, (n, k) in enumerate(pool) if n not in IMPLICIT), None)
            if hit is not None:
                pool.pop(hit)
                found_ids.add(id(s))
                (fnval if via_fn else loose).add(id(s))
    found = len(found_ids)
    # an `unsupported` reports the innermost call that contains it, if that call is not found
    reported = set()
    for f, l, c in uns:
        best = None
        for s in calls:
            if s["file"] != f or id(s) in found_ids:
                continue
            if (s["line"], s["col"]) <= (l, c) <= (s["end_line"], s["end_col"]):
                if best is None or (s["line"], s["col"]) > (best["line"], best["col"]):
                    best = s
        if best:
            reported.add(id(best))
    mismatches, allowed, n_reported = [], collections.Counter(), 0
    for s in calls:
        if id(s) in found_ids:
            continue
        if id(s) in reported:
            n_reported += 1
            continue
        name = next((n for n, (pred, _) in ALLOW.items() if pred(s)), None)
        if name:
            allowed[name] += 1
        else:
            mismatches.append(s)
    funcs = {(m["file"], m["owner_line"], m["owner"]) for m in mismatches}
    start_count = collections.Counter(pos_of(s) for s in calls)
    key_count = collections.Counter(pos_of(s) + (s["name"],) for s in calls)
    return {
        "syntax_calls": len(calls),
        "found": found,
        "found_fn_value": len(fnval),
        "found_loose": len(loose),
        "call_unsupported": n_reported,
        "call_mismatch": len(funcs),
        "mismatches": mismatches,
        "allowed": dict(allowed),
        "excluded": excl,
        "macro_opaque": opaque,
        "shared_start": sum(1 for s in calls if start_count[pos_of(s)] > 1),
        "weak": sum(1 for s in calls if key_count[pos_of(s) + (s["name"],)] > 1),
        "ir_calls": sum(len(v) for v in ir_calls.values()),
        "parse_errors": sorted({s["file"] for s in syntax if s.get("kind") == "parse_error"}),
        "files_without_ir": sorted(f for f in by_file if f not in files),
    }


def run_syntax(directory):
    env = dict(os.environ)
    cmd = [env.get("RIVUS_CARGO", "cargo"), "run", "--offline", "--release", "--quiet", "--manifest-path", os.path.join(HERE, "syntax", "Cargo.toml"), "--", os.path.abspath(directory)]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("callcheck: the syntax side failed: " + r.stderr.strip()[-400:])
    return r.stdout


def report(r, out=sys.stdout):
    print("syntax_calls %d" % r["syntax_calls"], file=out)
    print("found %d" % r["found"], file=out)
    print("found_fn_value %d" % r["found_fn_value"], file=out)
    print("found_loose %d" % r["found_loose"], file=out)
    print("call_unsupported %d" % r["call_unsupported"], file=out)
    print("allowed %s" % (json.dumps(r["allowed"], sort_keys=True) if r["allowed"] else "none"), file=out)
    print("excluded %s" % (json.dumps(r["excluded"], sort_keys=True) if r["excluded"] else "none"), file=out)
    print("macro_opaque %s" % (json.dumps(r["macro_opaque"], sort_keys=True) if r["macro_opaque"] else "0"), file=out)
    print("shared_start %d" % r["shared_start"], file=out)
    print("weak %d" % r["weak"], file=out)
    print("ir_calls %d" % r["ir_calls"], file=out)
    print("call_mismatch %d" % r["call_mismatch"], file=out)
    for f in r["parse_errors"]:
        print("parse_error %s" % f, file=out)
    for f in r["files_without_ir"]:
        print("file_without_ir %s" % f, file=out)
    for m in r["mismatches"]:
        print("mismatch %s:%d:%d %s `%s` in `%s`" % (m["file"], m["line"], m["col"], m["kind"], m["name"], m["owner"]), file=out)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("dir", nargs="?")
    ap.add_argument("--syntax", help="the output of syntax/ (instead of running it)")
    ap.add_argument("--ir", required=True, help="the IR of the extractor (this tool does not run the extractor)")
    ap.add_argument("--only", action="append", default=[], help="a path prefix under DIR (repeatable): only the files under it are checked; the zoo has files that are not a cell (the harness)")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args(argv)
    if not a.dir and not a.syntax:
        ap.error("a directory, or --syntax")
    syntax = parse_syntax(open(a.syntax).read() if a.syntax else run_syntax(a.dir))
    if a.only:
        syntax = [s for s in syntax if any(s["file"].startswith(p) for p in a.only)]
    calls, uns, files = parse_ir(open(a.ir).read())
    r = compare(syntax, calls, uns, files)
    if a.json:
        json.dump(r, sys.stdout, indent=1)
        print()
    else:
        report(r)
    return 0


if __name__ == "__main__":
    sys.exit(main())
