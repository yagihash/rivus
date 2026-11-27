#!/usr/bin/env python3
"""Tests of bench/rust-callcheck/callcheck.py (python3 bench/test_rust_callcheck.py): fixtures, and the syntax side on small sources
(needs `cargo` with the crates of bench/rust-callcheck/syntax available offline; skipped if not).

The check of the check (the tool must find a call that the extractor dropped, or "0 mismatch" says nothing):
`test_dropped_call_is_found` holds an IR from which one call is left out, and the tool must report it. With
CALLCHECK_IR_DIR=<dir with curated.ir.jsonl> (the file that `RIVUS_SAVE_IR=<dir> go test ./cmd/rivus -run TestSaveRustIRForCallcheck`
writes) the same is done on the real IR of bench/rust-curated: no mismatch as it is, and one `call` taken out of it gives one."""
import json, os, shutil, subprocess, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, os.path.join(HERE, "rust-callcheck"))
import callcheck as c


def syn(file, line, col, name="f", kind="call", owner="o", owner_line=1, excl="", value=False, end=None):
    return {"file": file, "kind": kind, "line": line, "col": col, "end_line": line, "end_col": end or col + 5, "name": name,
            "owner": owner, "owner_line": owner_line, "excl": excl, "value_callee": value, "in_macro": False}


def ir(*instrs):
    """One function (in a.rs) with the instructions (op, line, col, symbol, kind)."""
    items = []
    for t in instrs:
        op, line, col = t[:3]
        i = {"op": op, "pos": {"line": line, "col": col, "offset": line * 100000 + col}}
        if op == "call":
            i["callee"] = {"kind": t[4] if len(t) > 4 else "static", "symbol": t[3]}
        items.append(i)
    return "\n".join([json.dumps({"schema_version": "rivus.ir/1"}), json.dumps({"kind": "func", "func": {"id": "f", "file": "a.rs", "blocks": [{"id": 0, "instrs": items}, {"id": 1}]}})])


def run(syntax, text):
    calls, uns, files = c.parse_ir(text)
    return c.compare(syntax, calls, uns, files)


class CompareTest(unittest.TestCase):
    def test_all_found(self):
        r = run([syn("a.rs", 3, 5, "parse"), syn("a.rs", 4, 9, "len", kind="method")], ir(("call", 3, 5, "k::parse"), ("call", 4, 9, "std::vec::Vec::len")))
        self.assertEqual((r["call_mismatch"], r["found"], r["syntax_calls"]), (0, 2, 2))

    def test_dropped_call_is_found(self):
        # the second call is not in the IR: the tool must say so, and only for its function
        r = run([syn("a.rs", 3, 5, "a", owner_line=1), syn("a.rs", 9, 5, "b", owner="g", owner_line=7)], ir(("call", 3, 5, "k::a"), ("copy", 9, 5)))
        self.assertEqual(r["call_mismatch"], 1)
        self.assertEqual([(m["file"], m["line"], m["name"]) for m in r["mismatches"]], [("a.rs", 9, "b")])

    def test_chain_is_told_apart_by_name(self):
        # `x.f().g()`: both start at x. g is dropped, f is kept: the position alone would say "found"
        s = [syn("a.rs", 3, 5, "f", kind="method"), syn("a.rs", 3, 5, "g", kind="method")]
        r = run(s, ir(("call", 3, 5, "k::f")))
        self.assertEqual([m["name"] for m in r["mismatches"]], ["g"])
        r = run(s, ir(("call", 3, 5, "k::f"), ("call", 3, 5, "k::g")))
        self.assertEqual((r["call_mismatch"], r["shared_start"], r["weak"]), (0, 2, 0))

    def test_same_name_at_one_position_is_compared_by_number(self):
        # `x.f().f()`: the name does not tell them apart (weak); one call of f in the IR is one call short
        s = [syn("a.rs", 3, 5, "f", kind="method"), syn("a.rs", 3, 5, "f", kind="method")]
        r = run(s, ir(("call", 3, 5, "k::f")))
        self.assertEqual((r["call_mismatch"], len(r["mismatches"]), r["weak"]), (1, 1, 2))
        r = run(s, ir(("call", 3, 5, "k::f"), ("call", 3, 5, "k::f")))
        self.assertEqual(r["call_mismatch"], 0)

    def test_mismatch_counts_functions_not_calls(self):
        r = run([syn("a.rs", 3, 5, owner_line=1), syn("a.rs", 4, 5, owner_line=1), syn("a.rs", 9, 5, owner_line=5)], ir(("call", 99, 1, "k::z")))
        self.assertEqual((r["call_mismatch"], len(r["mismatches"])), (2, 3))

    def test_other_instruction_at_the_position_is_not_a_call(self):
        r = run([syn("a.rs", 3, 5)], ir(("field_load", 3, 5)))
        self.assertEqual(r["call_mismatch"], 1)

    def test_same_position_in_another_file_is_not_the_call(self):
        text = ir(("call", 3, 5, "k::f")).replace('"file": "a.rs"', '"file": "b.rs"')
        r = run([syn("a.rs", 3, 5)], text)
        self.assertEqual(r["call_mismatch"], 1)

    def test_a_call_in_the_ir_without_syntax_is_not_a_mismatch(self):
        # `?`, operators, auto-deref, what a macro makes: in the IR with no argument list in the source
        r = run([syn("a.rs", 3, 5, "f")], ir(("call", 3, 5, "k::f"), ("call", 3, 9, "std::ops::Deref::deref"), ("call", 8, 1, "std::ops::Try::branch")))
        self.assertEqual((r["call_mismatch"], r["ir_calls"]), (0, 3))

    def test_call_of_a_function_value(self):
        # `f(x)` with f a local: the IR has Fn::call, or the call of the function that f is known to be
        v = syn("a.rs", 3, 5, "f", value=True)
        self.assertEqual(run([v], ir(("call", 3, 5, "std::ops::Fn::call")))["found_fn_value"], 1)
        self.assertEqual(run([v], ir(("call", 3, 5, "k::Printer::run")))["found_loose"], 1)
        self.assertEqual(run([v], ir(("call", 3, 5, "std::ops::Try::branch")))["call_mismatch"], 1)  # an implicit call is not it
        # a call of a function that is not a local is not found by another name
        self.assertEqual(run([syn("a.rs", 3, 5, "f")], ir(("call", 3, 5, "std::ops::Fn::call")))["call_mismatch"], 1)

    def test_closure_call_is_not_taken_twice(self):
        # two calls of values at one position, one call in the IR
        s = [syn("a.rs", 3, 5, "f", value=True), syn("a.rs", 3, 5, "g", value=True)]
        self.assertEqual(run(s, ir(("call", 3, 5, "std::ops::Fn::call")))["call_mismatch"], 1)

    def test_file_the_ir_does_not_name(self):
        r = run([syn("a.rs", 3, 5, "f"), syn("z.rs", 3, 5, "f")], ir(("call", 3, 5, "k::f")))
        self.assertEqual((r["call_mismatch"], r["files_without_ir"]), (1, ["z.rs"]))

    def test_unsupported_is_reported_not_missing(self):
        r = run([syn("a.rs", 3, 5, "f", end=20)], ir(("unsupported", 3, 12)))
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 1))

    def test_unsupported_binds_to_the_inner_call(self):
        # a.b(c()).d(): the unsupported of c() does not report d()
        s = [syn("a.rs", 3, 1, "d", kind="method", end=30), syn("a.rs", 3, 8, "c", end=12)]
        r = run(s, ir(("unsupported", 3, 10)))
        self.assertEqual((r["call_mismatch"], r["call_unsupported"], [m["name"] for m in r["mismatches"]]), (1, 1, ["d"]))

    def test_call_wins_over_unsupported(self):
        r = run([syn("a.rs", 3, 5, "f")], ir(("call", 3, 5, "k::f"), ("unsupported", 3, 5)))
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 0))

    def test_excluded_calls_are_counted_by_kind(self):
        s = [syn("a.rs", 3, 5, "Some", excl="ctor"), syn("a.rs", 4, 5, "f", excl="ctor"), syn("a.rs", 5, 5, "g", excl="unreachable"),
             syn("a.rs", 6, 5, "h", excl="const_init"), syn("a.rs", 7, 5, "i", excl="cfg_test"), syn("a.rs", 8, 5, "j", excl="cfg")]
        r = run(s, ir(("call", 99, 1, "k::z")))
        self.assertEqual((r["call_mismatch"], r["syntax_calls"], r["excluded"]), (0, 0, {"ctor": 2, "unreachable": 1, "const_init": 1, "cfg_test": 1, "cfg": 1}))

    def test_opaque_macros_are_counted(self):
        s = [{"file": "a.rs", "kind": "macro", "name": "m", "args_parse": False, "excl": ""}, {"file": "a.rs", "kind": "macro", "name": "macro_rules", "args_parse": False, "excl": "macro_rules"},
             {"file": "a.rs", "kind": "macro", "name": "vec", "args_parse": True, "excl": ""}]
        r = run(s, ir(("call", 99, 1, "k::z")))
        self.assertEqual(r["macro_opaque"], {"arguments_not_expressions": 1, "macro_rules": 1})


class AllowTest(unittest.TestCase):
    def test_every_entry_has_a_reason(self):
        for name, (pred, reason) in c.ALLOW.items():
            self.assertTrue(callable(pred), name)
            self.assertGreater(len(reason.strip()), 20, name)
        for name, reason in c.EXCLUDED.items():
            self.assertGreater(len(reason.strip()), 20, name)

    def test_slice_len_is_allowed_and_counted_apart(self):
        r = run([syn("a.rs", 3, 5, "len", kind="method"), syn("a.rs", 4, 5, "push", kind="method")], ir(("call", 99, 1, "k::z")))
        self.assertEqual((r["call_mismatch"], r["allowed"]), (1, {"slice_len": 1}))


class ParseTest(unittest.TestCase):
    def test_instruction_without_position_is_skipped(self):
        text = json.dumps({"kind": "func", "func": {"file": "a.rs", "blocks": [{"instrs": [{"op": "call"}, {"op": "call", "pos": {"line": 2, "col": 3}, "callee": {"symbol": "k::f", "kind": "static"}}]}]}})
        calls, uns, files = c.parse_ir(text)
        self.assertEqual(dict(calls), {("a.rs", 2, 3): [("f", "static")]})

    def test_callee_names(self):
        self.assertEqual([c.callee_name(s) for s in ("a::b::f", "<W<T> as Tr>::m", "a::g::{closure#0}", "", None)], ["f", "m", "{closure}", "", ""])


SRC = r'''
fn plain(x: i32) -> Option<i32> {
    let a = f(x);
    let b = (g(a)).h();
    let c = Some(x);
    let cb = |y| y;
    cb(1);
    if false { never(); }
    println!("{}", w(x));
    vec![u(1), 2];
    vec![z(1); 3];
    (a.k())();
    Some(a)
}
fn after() {
    return;
    late();
}
const C: i32 = k();
#[cfg(test)]
mod tests { fn t() { in_test(); } }
fn feature() {
    #[cfg(feature = "x")]
    { off(); }
}
macro_rules! m { () => { inner() } }
'''


def have_cargo():
    return shutil.which(os.environ.get("RIVUS_CARGO", "cargo")) is not None


@unittest.skipUnless(have_cargo(), "cargo is not available")
class SyntaxSideTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.dir = tempfile.mkdtemp()
        with open(os.path.join(cls.dir, "a.rs"), "w") as f:
            f.write(SRC)
        try:
            cls.recs = c.parse_syntax(c.run_syntax(cls.dir))
        except SystemExit as e:
            raise unittest.SkipTest(str(e))

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.dir, ignore_errors=True)

    def find(self, name, kind=None):
        r = [s for s in self.recs if s.get("name") == name and (kind is None or s["kind"] == kind)]
        self.assertEqual(len(r), 1, (name, r))
        return r[0]

    def test_positions_are_the_start_of_the_call(self):
        f = self.find("f")
        self.assertEqual((f["line"], f["col"], f["excl"]), (3, 13, ""))
        # `(g(a)).h()`: rustc gives g(a) the span of the parentheses, so the calls of g and h both start at the "("
        self.assertEqual((self.find("g")["col"], self.find("h")["col"]), (13, 13))
        # `(a.k())()`: the method call is the parenthesised expression, so it starts at the "(" as well
        self.assertEqual(self.find("k", "method")["col"], 5)

    def test_macro_arguments_are_counted(self):
        self.assertEqual(self.find("w")["excl"], "")
        self.assertEqual(self.find("u")["excl"], "")
        self.assertEqual([s for s in self.recs if s.get("name") == "z"], [])  # `vec![z(1); 3]` is not a list of expressions: not seen

    def test_excluded_forms(self):
        self.assertEqual({s["excl"] for s in self.recs if s.get("name") == "Some"}, {"ctor"})
        self.assertEqual(self.find("never")["excl"], "unreachable")
        self.assertEqual(self.find("late")["excl"], "unreachable")
        self.assertEqual(self.find("k", "call")["excl"], "const_init")
        self.assertEqual(self.find("in_test")["excl"], "cfg_test")
        self.assertEqual(self.find("off")["excl"], "cfg")

    def test_local_callee_is_a_value(self):
        self.assertTrue(self.find("cb")["value_callee"])
        self.assertFalse(self.find("f")["value_callee"])

    def test_opaque_macros(self):
        r = c.compare(self.recs, {}, [], {"a.rs"})
        self.assertEqual(r["macro_opaque"], {"arguments_not_expressions": 1, "macro_rules": 1})  # vec![z(1); 3] and macro_rules! m


@unittest.skipUnless(os.environ.get("CALLCHECK_IR_DIR"), "CALLCHECK_IR_DIR=<dir with curated.ir.jsonl> uses the real IR of bench/rust-curated")
class RealIRTest(unittest.TestCase):
    """The check of the check, on the real IR: none as it is, one when a `call` is taken out of it."""

    def test_real_ir_and_a_dropped_call(self):
        ir_path = os.path.join(os.environ["CALLCHECK_IR_DIR"], "curated.ir.jsonl")
        syntax = c.parse_syntax(c.run_syntax(os.path.join(ROOT, "bench", "rust-curated")))
        text = open(ir_path).read()
        r = run(syntax, text)
        self.assertEqual(r["call_mismatch"], 0, r["mismatches"][:3])
        self.assertGreater(r["syntax_calls"], 500)
        # take out the call of `source` that errors.rs:20 makes (a function of the crate: it is in the syntax at that position)
        out, dropped = [], 0
        for line in text.splitlines():
            o = json.loads(line) if line.startswith("{") else None
            if o and "func" in o and isinstance(o["func"], dict) and o["func"].get("file") == "src/errors.rs":
                for b in o["func"].get("blocks") or []:
                    keep = []
                    for i in b.get("instrs") or []:
                        p = i.get("pos") or {}
                        if i.get("op") == "call" and p.get("line") == 20 and i["callee"].get("symbol") == "curated::source":
                            dropped += 1
                            continue
                        keep.append(i)
                    b["instrs"] = keep
                line = json.dumps(o)
            out.append(line)
        self.assertEqual(dropped, 1)
        r2 = run(syntax, "\n".join(out))
        self.assertEqual((r2["call_mismatch"], [(m["file"], m["line"], m["name"]) for m in r2["mismatches"]]), (1, [("src/errors.rs", 20, "source")]))


if __name__ == "__main__":
    unittest.main()
