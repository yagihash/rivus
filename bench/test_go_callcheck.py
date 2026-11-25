#!/usr/bin/env python3
"""Tests of bench/go-callcheck/callcheck.py (python3 bench/test_go_callcheck.py): fixtures only, nothing is run.

The check of the check (the tool must find a call that the extractor dropped, or "0 mismatch" says nothing):
`test_dropped_call_is_found` holds an IR from which one call is left out, and the tool must report it.
CALLCHECK_E2E=1 does it against the real extractor: a copy of the sources of `rivus` (made by `git archive` in a temporary
directory) that leaves out the static calls of methods must give a mismatch."""
import json, os, subprocess, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "go-callcheck"))
import callcheck as c

ROOT = os.path.dirname(HERE)


def syn(file, offset, kind="call", name="", owner="f", owner_line=1, start=None, line=1, dead=""):
    r = {"file": file, "offset": offset, "start": offset - 3 if start is None else start, "line": line, "kind": kind, "name": name, "owner": owner, "owner_line": owner_line}
    if dead:
        r["dead"] = dead
    return r


def ir(*instrs, func_file=""):
    """One function with the instructions (op, file, offset); a file "" is the file of the function."""
    items = [{"op": op, "pos": ({"file": f, "offset": o} if f else {"offset": o})} for op, f, o in instrs]
    fn = {"id": "f", "blocks": [{"id": 0, "instrs": items}, {"id": 1}]}
    if func_file:
        fn["file"] = func_file
    return "\n".join([json.dumps({"schema_version": "rivus.ir/1"}), json.dumps({"kind": "func", "func": fn})])


class CompareTest(unittest.TestCase):
    def test_all_found(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10), ("call", "a.go", 20), ("copy", "a.go", 5)))
        r = c.compare([syn("a.go", 10), syn("a.go", 20)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"], r["call_dead"], r["syntax_calls"]), (0, 0, 0, 2))

    def test_dropped_call_is_found(self):
        # the second call is not in the IR: the tool must say so, and only for its function
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10), ("copy", "a.go", 20)))
        r = c.compare([syn("a.go", 10, owner_line=1), syn("a.go", 20, owner="g", owner_line=7)], calls, uns, files)
        self.assertEqual(r["call_mismatch"], 1)
        self.assertEqual([(m["file"], m["offset"]) for m in r["mismatches"]], [("a.go", 20)])

    def test_mismatch_counts_functions_not_calls(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 99)))
        r = c.compare([syn("a.go", 10, owner_line=1), syn("a.go", 20, owner_line=1), syn("a.go", 30, owner_line=5)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], len(r["mismatches"])), (2, 3))

    def test_other_instruction_at_the_position_is_not_a_call(self):
        calls, uns, files = c.parse_ir(ir(("field_load", "a.go", 10)))
        self.assertEqual(c.compare([syn("a.go", 10)], calls, uns, files)["call_mismatch"], 1)

    def test_same_offset_in_another_file_is_not_the_call(self):
        calls, uns, files = c.parse_ir(ir(("call", "b.go", 10)))
        self.assertEqual(c.compare([syn("a.go", 10)], calls, uns, files)["call_mismatch"], 1)

    def test_position_of_the_function_file(self):
        # an instruction whose position has no file is in the file of its function
        calls, uns, files = c.parse_ir(ir(("call", "", 10), func_file="a.go"))
        self.assertEqual(calls, {("a.go", 10)})
        self.assertEqual(c.compare([syn("a.go", 10)], calls, uns, files)["call_mismatch"], 0)

    def test_go_and_defer_are_at_the_keyword(self):
        # the "(" of `go f(x)` is not where the IR has the call: a syntax record at the "(" is a mismatch
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10)))
        self.assertEqual(c.compare([syn("a.go", 10, kind="go")], calls, uns, files)["call_mismatch"], 0)
        self.assertEqual(c.compare([syn("a.go", 15, kind="go", start=10)], calls, uns, files)["call_mismatch"], 1)

    def test_unsupported_is_reported_not_missing(self):
        calls, uns, files = c.parse_ir(ir(("unsupported", "a.go", 10), ("unsupported", "a.go", 31)))
        r = c.compare([syn("a.go", 10), syn("a.go", 40, start=30)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 2))

    def test_call_wins_over_unsupported(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10), ("unsupported", "a.go", 10)))
        r = c.compare([syn("a.go", 10)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 0))

    def test_call_with_no_syntax_is_not_a_mismatch(self):
        # the implicit calls (rt.rundefers, the body of a range over func) have no argument list in the source
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10), ("call", "a.go", 50)))
        self.assertEqual(c.compare([syn("a.go", 10)], calls, uns, files)["call_mismatch"], 0)

    def test_file_the_ir_does_not_name(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10)))
        r = c.compare([syn("a.go", 10), syn("z.go", 5)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["files_without_ir"]), (1, ["z.go"]))


class DeadTest(unittest.TestCase):
    def test_dead_call_is_counted_apart(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10)))
        r = c.compare([syn("a.go", 10), syn("a.go", 20, dead="unreachable"), syn("a.go", 30, dead="discarded_closure")], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_dead"], r["dead"]), (0, 2, {"unreachable": 1, "discarded_closure": 1}))

    def test_dead_call_that_the_ir_has_is_found(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 20)))
        r = c.compare([syn("a.go", 20, dead="unreachable")], calls, uns, files)
        self.assertEqual((r["found"], r["call_dead"]), (1, 0))

    def test_a_reason_the_tool_does_not_know_is_a_mismatch(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10)))
        self.assertEqual(c.compare([syn("a.go", 20, dead="because")], calls, uns, files)["call_mismatch"], 1)


class AllowTest(unittest.TestCase):
    def test_every_entry_has_a_reason(self):
        for name, (pred, reason) in c.ALLOW.items():
            self.assertTrue(callable(pred), name)
            self.assertGreater(len(reason.strip()), 20, name)
        for name, reason in c.DEAD.items():
            self.assertGreater(len(reason.strip()), 20, name)

    def test_builtins_that_are_not_calls_are_allowed_by_name(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 1)))
        r = c.compare([syn("a.go", 10, kind="builtin", name="make"), syn("a.go", 20, kind="builtin", name="panic"), syn("a.go", 30, kind="builtin", name="append")], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["allowed"]), (1, {"builtin_make": 1, "builtin_panic": 1}))

    def test_allowed_call_is_counted_apart(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 99)))
        old = dict(c.ALLOW)
        c.ALLOW["fixture"] = (lambda s: s["kind"] == "go", "a fixture entry, for this test only: a reason is required")
        try:
            r = c.compare([syn("a.go", 10, kind="go"), syn("a.go", 20)], calls, uns, files)
        finally:
            c.ALLOW.clear(); c.ALLOW.update(old)
        self.assertEqual((r["call_mismatch"], r["allowed"]), (1, {"fixture": 1}))


class ParseTest(unittest.TestCase):
    def test_instruction_without_position_is_skipped(self):
        text = json.dumps({"kind": "func", "func": {"file": "a.go", "blocks": [{"instrs": [{"op": "call"}, {"op": "call", "pos": {"offset": 3}}]}]}})
        self.assertEqual(c.parse_ir(text)[0], {("a.go", 3)})

    def test_syntax_lines(self):
        self.assertEqual(len(c.parse_syntax(json.dumps(syn("a.go", 4)) + "\n\n")), 1)

    def test_report_has_the_lines_of_the_ledger(self):
        import io
        calls, uns, files = c.parse_ir(ir(("call", "a.go", 10)))
        out = io.StringIO()
        c.report(c.compare([syn("a.go", 10), syn("a.go", 20, dead="unreachable")], calls, uns, files), out)
        self.assertIn("call_mismatch 0\n", out.getvalue())
        self.assertIn("call_dead 1\n", out.getvalue())
        self.assertIn("call_unsupported 0\n", out.getvalue())


FIXTURE = """package fixture

type T struct{ n int }

func (t *T) M(x int) int { return t.n + x }

func F(t *T) int {
	a := t.M(1)
	go t.M(2)
	defer t.M(3)
	g := func(x int) int { return x }
	_ = len("abc")
	_ = make([]int, 1)
	return a + g(4)
}

func After() {
	return
	println("never")
}

func Unused() {
	_ = func() { println("never") }
}
"""


@unittest.skipUnless(os.environ.get("CALLCHECK_E2E"), "CALLCHECK_E2E=1 runs the real extractor (builds rivus twice; a minute)")
class RealExtractorTest(unittest.TestCase):
    """The check of the check, for real: the tree gives no mismatch on a small module, and a copy of the tree that leaves out
    the static calls of methods (made from `git archive HEAD internal cmd go.mod go.sum` in a temporary directory; this tree is
    not touched) gives one."""

    def test_dropped_static_method_call_is_found(self):
        with tempfile.TemporaryDirectory() as t:
            mod, cp = os.path.join(t, "mod"), os.path.join(t, "tree")
            os.makedirs(mod)
            os.makedirs(cp)
            open(os.path.join(mod, "go.mod"), "w").write("module fixture\n\ngo 1.23\n")
            open(os.path.join(mod, "f.go"), "w").write(FIXTURE)
            arc = subprocess.run(["git", "archive", "HEAD", "internal", "cmd", "go.mod", "go.sum"], cwd=ROOT, capture_output=True, check=True).stdout
            subprocess.run(["tar", "-x", "-C", cp], input=arc, check=True)
            path = os.path.join(cp, "internal", "frontend", "goir", "ops.go")
            src = open(path).read()
            old = "\t\tcase *ssa.Function:\n"
            self.assertEqual(src.count(old), 1)
            open(path, "w").write(src.replace(old, old + "\t\t\tif fn.Signature.Recv() != nil {\n\t\t\t\treturn\n\t\t\t}\n", 1))
            syntax = c.parse_syntax(c.run_syntax(mod))
            good = c.compare(syntax, *c.parse_ir(c.run_extractor(mod, ROOT)))
            bad = c.compare(syntax, *c.parse_ir(c.run_extractor(mod, cp)))
        self.assertEqual(good["call_mismatch"], 0, good["mismatches"])
        self.assertEqual(good["call_dead"], 2)
        self.assertGreater(bad["call_mismatch"], 0)
        self.assertTrue({m["kind"] for m in bad["mismatches"]} >= {"call", "go", "defer"})


if __name__ == "__main__":
    unittest.main()
