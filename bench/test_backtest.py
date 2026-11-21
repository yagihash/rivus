#!/usr/bin/env python3
"""Tests of bench/backtest.py and bench/ledger.py (python3 bench/test_backtest.py). Synthetic input only; nothing is analysed."""
import json, os, subprocess, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
BT = os.path.join(HERE, "backtest.py")


def run(*args):
    return subprocess.run([sys.executable, BT] + list(args), capture_output=True, text=True)


class Docs(unittest.TestCase):
    def test_docs_are_made_from_the_rows(self):
        """BACKTEST.md's table and the head of LEDGER.md are the ones that bench/backtest-rows.json makes (the only source)."""
        r = run("docs", os.path.join(HERE, "backtest-rows.json"), os.path.join(HERE, "BACKTEST.md"), os.path.join(HERE, "ledger-header.md"))
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_every_row_has_what_it_needs(self):
        rows = json.load(open(os.path.join(HERE, "backtest-rows.json")))
        for r in rows:
            self.assertIn(r["status"], ("verified", "unverified", "optional", "miss"), r["id"])
            if r["status"] == "verified":
                self.assertTrue(r.get("verified_on"), "row %s is verified: the day it was" % r["id"])
                self.assertTrue(r["bad_ref"] and r["good_ref"], r["id"])
            if r["status"] == "unverified":
                self.assertTrue(r.get("unverified_why") and r.get("verified_when"), "row %s: why, and what makes it verified" % r["id"])


def go_test_json(path, tests):
    """A `go test -json` file: tests is [(name, "pass"|"fail", [output lines])]."""
    with open(path, "w") as f:
        for name, st, lines in tests:
            for l in lines:
                f.write(json.dumps({"Action": "output", "Test": name, "Output": l + "\n"}) + "\n")
            f.write(json.dumps({"Action": st, "Test": name}) + "\n")


class ExitCodes(unittest.TestCase):
    def judge(self, tests):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "o.json")
            go_test_json(p, tests)
            r = run("judge", p)
            return r.returncode, r.stdout

    def test_green_is_0(self):
        self.assertEqual(self.judge([("TestScaleSynth/a", "pass", [])])[0], 0)

    def test_red_is_1(self):
        rc, out = self.judge([("TestScaleSynth/a", "fail", ["    x_test.go:1: unexpected flow (must stay quiet): a.go:1 -> b.go:2"])])
        self.assertEqual(rc, 1)
        self.assertIn("RED", out)
        self.assertIn("FAKE 1", out)

    def test_an_unrecognized_failure_is_an_error_and_counted(self):
        rc, out = self.judge([("TestScaleSynth/a", "fail", ["panic: runtime error: index out of range"])])
        self.assertEqual(rc, 1)
        self.assertIn("ERROR", out)
        self.assertNotIn("UNJUDGED", out)

    def test_an_old_build_that_cannot_read_the_input_is_unjudged_and_2(self):
        rc, out = self.judge([("TestScaleSynth/a", "fail", ["analyze: unknown flag -parts"])])
        self.assertEqual(rc, 2)
        self.assertIn("UNJUDGED", out)

    def test_a_red_beside_an_unjudged_is_still_1(self):
        rc, _ = self.judge([("TestScaleSynth/a", "fail", ["analyze: unknown flag -parts"]),
                            ("TestScaleSynth/b", "fail", ["    x_test.go:1: missing flow: a.go:1 -> b.go:2"])])
        self.assertEqual(rc, 1)

    def test_no_result_at_all_is_an_error(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "o.json")
            open(p, "w").write("not json\n")
            r = run("judge", p)
            self.assertEqual(r.returncode, 1)
            self.assertIn("ERROR", r.stdout)


class Det(unittest.TestCase):
    def binary(self, d, body):
        p = os.path.join(d, "fake-rivus")
        open(p, "w").write("#!/bin/sh\n" + body + "\n")
        os.chmod(p, 0o755)
        return p

    def test_a_crash_with_an_empty_stderr_is_an_error_not_a_traceback(self):
        with tempfile.TemporaryDirectory() as d:
            r = run("det", self.binary(d, "exit 3"), "1")
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertIn("ERROR", r.stdout)
            self.assertNotIn("Traceback", r.stderr)

    def test_an_old_build_that_cannot_read_the_rules_is_unjudged(self):
        with tempfile.TemporaryDirectory() as d:
            r = run("det", self.binary(d, "echo 'rules: unknown field parts' >&2; exit 2"), "1")
            self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
            self.assertIn("UNJUDGED", r.stdout)

    def test_the_same_output_is_0_and_a_different_one_is_1(self):
        with tempfile.TemporaryDirectory() as d:
            same = self.binary(d, 'echo \'{"stats":{"duration_ms":1},"findings":[]}\'')
            r = run("det", same, "1")
            self.assertEqual((r.returncode, r.stdout.split()[0]), (0, "SAME"), r.stdout + r.stderr)
        with tempfile.TemporaryDirectory() as d:
            diff = self.binary(d, 'echo "{\\"stats\\":{},\\"findings\\":[$RANDOM]}"')
            r = run("det", diff, "1")
            self.assertEqual((r.returncode, r.stdout.split()[0]), (1, "DIFFERENT:"), r.stdout + r.stderr)


class Ledger(unittest.TestCase):
    def make(self, root, name, n):
        raw = os.path.join(root, name, "raw")
        os.makedirs(raw)
        open(os.path.join(raw, "fixed-inputs"), "w").close()
        rep = {"findings": [{"source": {"position": {"file": "a.go", "line": i}}, "sink": {"position": {"file": "b.go", "line": i}}, "category": "c"} for i in range(n)],
               "stats": {"converged": True}, "uncertainties": []}
        json.dump(rep, open(os.path.join(raw, "b.base.json"), "w"))
        open(os.path.join(raw, "b.base.prov"), "w").write("in\text\tir\tabcdef1\tr\n")
        return os.path.join(root, name)

    def test_the_same_engine_and_inputs_twice_is_a_warning(self):
        with tempfile.TemporaryDirectory() as root:
            a, b = self.make(root, "one", 3), self.make(root, "two", 4)
            r = subprocess.run([sys.executable, os.path.join(HERE, "ledger.py"), "-ref", "abcdef1", a, b], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertIn("WARNING", r.stderr)
            self.assertIn("THE FINDINGS DIFFER", r.stderr)
            self.assertIn("abcdef1", r.stdout)

    def test_no_warning_when_there_is_one_run(self):
        with tempfile.TemporaryDirectory() as root:
            a = self.make(root, "one", 3)
            r = subprocess.run([sys.executable, os.path.join(HERE, "ledger.py"), "-ref", "abcdef1", a], capture_output=True, text=True)
            self.assertEqual(r.stderr, "")


if __name__ == "__main__":
    unittest.main()
