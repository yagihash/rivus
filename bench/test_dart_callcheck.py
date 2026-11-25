#!/usr/bin/env python3
"""Tests of bench/dart-callcheck/callcheck.py (python3 bench/test_dart_callcheck.py): fixtures only, nothing is run.

The check of the check (the tool must find a call that the extractor dropped, or "0 mismatch" says nothing):
`test_dropped_call_is_found` holds an IR from which one call is left out, and the tool must report it. The same was
done once against the real extractor (the procedure is in bench/README.md, "The check of the call positions")."""
import json, os, sys, unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "dart-callcheck"))
import callcheck as c


def syn(file, offset, kind="method", owner="f", owner_line=1, start=None, line=1):
    return {"file": file, "offset": offset, "start": offset - 3 if start is None else start, "line": line, "kind": kind, "owner": owner, "owner_line": owner_line}


def ir(*instrs):
    """One function with the instructions (op, file, offset)."""
    items = [{"op": op, "pos": {"file": f, "offset": o}} for op, f, o in instrs]
    return "\n".join([json.dumps({"schema_version": "rivus.ir/1"}), json.dumps({"kind": "func", "func": {"id": "f", "blocks": [{"id": 0, "instrs": items}, {"id": 1}]}})])


class CompareTest(unittest.TestCase):
    def test_all_found(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 10), ("call", "a.dart", 20), ("copy", "a.dart", 5)))
        r = c.compare([syn("a.dart", 10), syn("a.dart", 20)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"], r["syntax_calls"]), (0, 0, 2))

    def test_dropped_call_is_found(self):
        # the second call is not in the IR: the tool must say so, and only for its function
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 10), ("copy", "a.dart", 20)))
        r = c.compare([syn("a.dart", 10, owner_line=1), syn("a.dart", 20, owner="g", owner_line=7)], calls, uns, files)
        self.assertEqual(r["call_mismatch"], 1)
        self.assertEqual([(m["file"], m["offset"]) for m in r["mismatches"]], [("a.dart", 20)])

    def test_mismatch_counts_functions_not_calls(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 99)))
        r = c.compare([syn("a.dart", 10, owner_line=1), syn("a.dart", 20, owner_line=1), syn("a.dart", 30, owner_line=5)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], len(r["mismatches"])), (2, 3))

    def test_other_instruction_at_the_position_is_not_a_call(self):
        calls, uns, files = c.parse_ir(ir(("field_load", "a.dart", 10)))
        r = c.compare([syn("a.dart", 10)], calls, uns, files)
        self.assertEqual(r["call_mismatch"], 1)

    def test_same_offset_in_another_file_is_not_the_call(self):
        calls, uns, files = c.parse_ir(ir(("call", "b.dart", 10)))
        r = c.compare([syn("a.dart", 10)], calls, uns, files)
        self.assertEqual(r["call_mismatch"], 1)

    def test_unsupported_is_reported_not_missing(self):
        # at the "(" itself, or inside the callee part of the call (the start of the node .. the "(")
        calls, uns, files = c.parse_ir(ir(("unsupported", "a.dart", 10), ("unsupported", "a.dart", 31)))
        r = c.compare([syn("a.dart", 10), syn("a.dart", 40, start=30)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 2))

    def test_unsupported_binds_to_the_inner_call(self):
        # a.b(c()).d(): the unsupported of c() does not report d()
        calls, uns, files = c.parse_ir(ir(("unsupported", "a.dart", 10)))
        r = c.compare([syn("a.dart", 10, start=8), syn("a.dart", 30, start=1)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (1, 1))

    def test_call_wins_over_unsupported(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 10), ("unsupported", "a.dart", 10)))
        r = c.compare([syn("a.dart", 10)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["call_unsupported"]), (0, 0))

    def test_call_with_no_syntax_is_not_a_mismatch(self):
        # implicit calls (getters, operators, the callback of the platform) are in the IR with no argument list
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 10), ("call", "a.dart", 50)))
        r = c.compare([syn("a.dart", 10)], calls, uns, files)
        self.assertEqual(r["call_mismatch"], 0)

    def test_file_the_ir_does_not_name(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 10)))
        r = c.compare([syn("a.dart", 10), syn("z.dart", 5)], calls, uns, files)
        self.assertEqual((r["call_mismatch"], r["files_without_ir"]), (1, ["z.dart"]))


class AllowTest(unittest.TestCase):
    def test_every_entry_has_a_reason(self):
        for name, (pred, reason) in c.ALLOW.items():
            self.assertTrue(callable(pred), name)
            self.assertGreater(len(reason.strip()), 20, name)

    def test_allowed_call_is_counted_apart(self):
        calls, uns, files = c.parse_ir(ir(("call", "a.dart", 99)))
        old = dict(c.ALLOW)
        c.ALLOW["fixture"] = (lambda s: s["kind"] == "enum_constant", "a fixture entry, for this test only: a reason is required")
        try:
            r = c.compare([syn("a.dart", 10, kind="enum_constant"), syn("a.dart", 20)], calls, uns, files)
        finally:
            c.ALLOW.clear(); c.ALLOW.update(old)
        self.assertEqual((r["call_mismatch"], r["allowed"]), (1, {"fixture": 1}))


class ParseTest(unittest.TestCase):
    def test_instruction_without_position_is_skipped(self):
        text = json.dumps({"kind": "func", "func": {"blocks": [{"instrs": [{"op": "call"}, {"op": "call", "pos": {"file": "a.dart", "offset": 3}}]}]}})
        calls, uns, files = c.parse_ir(text)
        self.assertEqual(calls, {("a.dart", 3)})

    def test_syntax_lines(self):
        s = c.parse_syntax(json.dumps(syn("a.dart", 4)) + "\n\n")
        self.assertEqual(len(s), 1)


@unittest.skipUnless(os.environ.get("CALLCHECK_E2E"), "CALLCHECK_E2E=1 runs the real extractor (about 1 minute; needs `dart` and `dart pub get` in extractor/dart and bench/dart-curated)")
class RealExtractorTest(unittest.TestCase):
    """The check of the check, for real: a copy of the extractor that leaves out the calls of one form (a call of a function
    value) must make the tool report mismatches on bench/dart-curated, and the extractor of the tree must give none."""

    def test_dropped_form_is_found(self):
        import shutil, tempfile
        root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        curated = os.path.join(root, "bench", "dart-curated")
        with tempfile.TemporaryDirectory() as t:
            cp = os.path.join(t, "dart")
            shutil.copytree(os.path.join(root, "extractor", "dart"), cp)
            path = os.path.join(cp, "lib", "exprs.dart")
            src = open(path).read()
            old = "    case FunctionExpressionInvocation():\n      return _functionInvocation(f, e);"
            self.assertIn(old, src)
            open(path, "w").write(src.replace(old, "    case FunctionExpressionInvocation():\n      return f.nilConst(f.pos(e));"))
            syntax = c.parse_syntax(c.run_syntax(curated))
            good = c.compare(syntax, *c.parse_ir(c.run_extractor(curated, os.path.join(root, "extractor", "dart"))))
            bad = c.compare(syntax, *c.parse_ir(c.run_extractor(curated, cp)))
        self.assertEqual(good["call_mismatch"], 0)
        self.assertGreater(bad["call_mismatch"], 0)
        self.assertIn("function_expression", {m["kind"] for m in bad["mismatches"]})


if __name__ == "__main__":
    unittest.main()
