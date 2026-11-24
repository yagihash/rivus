#!/usr/bin/env python3
"""Tests of bench/heavy_ledger.py and of the rows that bench/deps_bench.py adds (python3 bench/test_heavy_ledger.py).
Fake reports and a fake rivus only: nothing is fetched or analysed."""
import argparse, io, json, os, stat, subprocess, sys, tempfile, unittest
from contextlib import redirect_stdout

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import deps_bench as d
import heavy_ledger as h

NEW = {"summary": {"memory_limit_bytes": 8 * 1024 ** 3, "memory_limit_source": "default", "memory_peak_bytes": 6 * 1024 ** 3,
                   "live_heap_max_bytes": 4 * 1024 ** 3, "gc_cpu_fraction": 0.12, "incomplete_reasons": []},
       "stats": {"converged": True, "func_timeouts": 0, "budget_exhausted": 3}, "findings": [{"category": "x"}] * 2}
OLD = {"summary": {"incomplete_reasons": ["timeout"]}, "stats": {"converged": False}, "findings": []}  # a binary without summary.memory_*


def args(path, no_ledger=False):
    return argparse.Namespace(ledger=path, no_ledger=no_ledger)


class Writer(unittest.TestCase):
    def setUp(self):
        self.t = tempfile.TemporaryDirectory()
        self.path = os.path.join(self.t.name, "ledger.tsv")

    def tearDown(self):
        self.t.cleanup()

    def lines(self):
        return open(self.path).read().splitlines()

    def test_row_added_with_header(self):
        d.add_to_ledger({"target": "memos", "kind": "passline", "result": "PASS", **h.report_columns(NEW)}, args(self.path), "abc1234")
        L = self.lines()
        self.assertEqual(L[0].split("\t"), h.COLUMNS)
        self.assertEqual(len(L), 2)
        row = dict(zip(h.COLUMNS, L[1].split("\t")))
        self.assertEqual((row["rivus_commit"], row["target"], row["memory_peak_bytes"], row["gc_cpu_fraction"], row["findings"], row["budget_exhausted"]),
                         ("abc1234", "memos", str(6 * 1024 ** 3), "0.12", "2", "3"))
        self.assertRegex(row["date"], r"^\d{4}-\d\d-\d\d$")

    def test_no_ledger_adds_nothing(self):
        d.add_to_ledger({"target": "memos", "kind": "passline"}, args(self.path, no_ledger=True), "abc1234")
        self.assertFalse(os.path.exists(self.path))

    def test_old_report_without_memory_fields_does_not_stop(self):
        c = h.report_columns(OLD)
        d.add_to_ledger({"target": "memos", "kind": "passline", "result": "FAIL", **{k: v for k, v in c.items() if v is not None}}, args(self.path), "abc1234")
        row = dict(zip(h.COLUMNS, self.lines()[1].split("\t")))
        self.assertEqual((row["memory_peak_bytes"], row["gc_cpu_fraction"], row["live_heap_max_bytes"], row["incomplete_reasons"]), ("", "", "", "timeout"))
        self.assertEqual(h.report_columns(None), {})
        self.assertEqual(h.report_columns({})["memory_peak_bytes"], None)

    def test_two_rows_have_the_same_number_of_columns(self):
        h.append_row({"target": "a", "result": "PASS\twith a tab"}, self.path)
        h.append_row({"target": "b"}, self.path)
        L = self.lines()
        self.assertEqual(len(L), 3)
        self.assertEqual({len(l.split("\t")) for l in L}, {len(h.COLUMNS)})

    def test_unknown_column_is_refused(self):
        with self.assertRaises(ValueError):
            h.append_row({"nope": 1}, self.path)

    def test_parallel_appends_do_not_break_lines(self):
        code = "import sys; sys.path.insert(0, %r); import heavy_ledger as h\nfor i in range(40): h.append_row({'target': 'p', 'settings': 'x' * 3000}, %r)" % (HERE, self.path)
        ps = [subprocess.Popen([sys.executable, "-c", code]) for _ in range(4)]
        for p in ps:
            self.assertEqual(p.wait(), 0)
        L = self.lines()
        self.assertEqual(len(L), 161)
        self.assertEqual({len(l.split("\t")) for l in L}, {len(h.COLUMNS)})
        self.assertEqual(sum(1 for l in L if l.startswith("date\t")), 1)


class Reader(unittest.TestCase):
    def test_margins(self):
        r = {"memory_limit_bytes": str(8 * 1024 ** 3), "memory_peak_bytes": str(6 * 1024 ** 3), "live_heap_max_bytes": str(4 * 1024 ** 3)}
        self.assertEqual(h.margins(r), (2 * 1024 ** 3, 4 * 1024 ** 3))
        self.assertEqual(h.margins({"memory_limit_bytes": "100"}), (None, None))
        self.assertEqual(h.margins({}), (None, None))

    def test_table_and_filters(self):
        with tempfile.TemporaryDirectory() as w:
            p = os.path.join(w, "l.tsv")
            for target, kind, c in (("memos", "passline", NEW), ("memos", "passline", OLD), ("gcp", "deps", NEW)):
                h.append_row({"date": "2026-11-23", "target": target, "kind": kind, **{k: v for k, v in h.report_columns(c).items() if v is not None}}, p)
            open(p, "a").write("# a comment line\n")
            out = io.StringIO()
            with redirect_stdout(out):
                self.assertEqual(h.main(["--ledger", p, "--kind", "passline", "--target", "memos"]), 0)
            L = out.getvalue().splitlines()
            self.assertEqual(len(L), 3)  # header and two rows
            self.assertIn("2.00", L[1])  # 8 GB - 6 GB
            self.assertIn("4.00", L[1])  # 8 GB - 4 GB (live heap)
            out = io.StringIO()
            with redirect_stdout(out):
                h.main(["--ledger", p, "--last", "1"])
            self.assertEqual(len(out.getvalue().splitlines()), 2)

    def test_missing_ledger_is_not_an_error(self):
        out = io.StringIO()
        with redirect_stdout(out):
            self.assertEqual(h.main(["--ledger", "/nonexistent/x.tsv"]), 0)


class DepsRun(unittest.TestCase):
    def test_measure_with_a_fake_rivus_gives_a_row(self):
        with tempfile.TemporaryDirectory() as w:
            fake = os.path.join(w, "rivus")
            open(fake, "w").write("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n" % json.dumps(NEW))
            os.chmod(fake, os.stat(fake).st_mode | stat.S_IXUSR)
            os.environ["RIVUS_HEAVY_LOCK"] = os.path.join(w, "lock")
            try:
                row, why = d.measure("go-s3", fake, 5, w)
            finally:
                del os.environ["RIVUS_HEAVY_LOCK"]
            self.assertIsNone(why)
            self.assertEqual((row["kind"], row["result"], row["converged"], row["exit_code"]), ("deps", "finished", True, 0))
            self.assertEqual(row["memory_peak_bytes"], 6 * 1024 ** 3)
            p = os.path.join(w, "l.tsv")
            d.add_to_ledger(row, args(p), "abc1234")
            self.assertEqual(len(open(p).read().splitlines()[1].split("\t")), len(h.COLUMNS))


class PasslineRun(unittest.TestCase):
    def test_passline_row_with_a_fake_rivus(self):
        with tempfile.TemporaryDirectory() as w:
            fake = os.path.join(w, "rivus")
            open(fake, "w").write("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n" % json.dumps(NEW))
            os.chmod(fake, os.stat(fake).st_mode | stat.S_IXUSR)
            os.environ["RIVUS_HEAVY_LOCK"] = os.path.join(w, "lock")
            os.environ["BENCH_OUT"] = os.path.join(w, "out")
            orig = d.prepare
            d.prepare = lambda name, t, tree, work, env: (work, env, None)
            t = {"lang": "go", "local": "bench/go-s3", "args": [], "time_limit_s": 60, "mem_cap_gb": 8, "decided": True, "commit": "c0ffee"}
            try:
                line, row = d.passline("fake", t, fake, w, d.out_dir(), 60, 8)
            finally:
                d.prepare = orig
                del os.environ["RIVUS_HEAVY_LOCK"], os.environ["BENCH_OUT"]
            self.assertTrue(line.startswith("PASS fake "))
            self.assertEqual((row["kind"], row["result"], row["settings"], row["target_commit"], row["findings"]), ("passline", "PASS", "default", "c0ffee", 2))
            p = os.path.join(w, "l.tsv")
            d.add_to_ledger(row, args(p), "abc1234")
            self.assertEqual(len(open(p).read().splitlines()[1].split("\t")), len(h.COLUMNS))


    def test_passline_score_row_has_no_finding_count(self):
        truth = {"sites": [{"source_func": "a.A", "sink_file": "s.go", "sink_line": 1, "label": "vuln", "category": "ssrf", "vuln": 1}]}
        rep = dict(NEW, findings=[{"category": "ssrf", "source": {"func": "a.A"}, "sink": {"position": {"file": "s.go", "line": 1}}}] * 2)
        with tempfile.TemporaryDirectory() as w:
            fake = os.path.join(w, "rivus")
            open(fake, "w").write("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n" % json.dumps(rep))
            os.chmod(fake, os.stat(fake).st_mode | stat.S_IXUSR)
            os.makedirs(os.path.join(w, "bench"))
            json.dump(truth, open(os.path.join(w, "truth.json"), "w"))
            os.environ["RIVUS_HEAVY_LOCK"] = os.path.join(w, "lock")
            os.environ["BENCH_OUT"] = os.path.join(w, "out")
            orig, root = d.prepare, d.ROOT
            d.prepare = lambda name, t, tree, work, env: (work, env, None)
            outdir = d.out_dir()
            d.ROOT = w
            t = {"lang": "go", "local": "bench", "args": ["-no-project-rules"], "time_limit_s": 600, "mem_cap_gb": 8, "decided": True,
                 "commit": "c0ffee", "detect": {"id": "x", "mode": "score", "truth": "truth.json"}}
            try:
                line, row = d.passline("fakegitea", t, fake, w, outdir, 600, 8)
            finally:
                d.prepare, d.ROOT = orig, root
                del os.environ["RIVUS_HEAVY_LOCK"], os.environ["BENCH_OUT"]
            self.assertEqual(line, line.split("detected=")[0] + "detected=1/1 quiet_after_fix=n/a coarsened=0/0")
            self.assertTrue(line.startswith("PASS fakegitea "))
            self.assertEqual(row["detected"], "1/1")
            self.assertNotIn("findings", row)
            p = os.path.join(w, "l.tsv")
            d.add_to_ledger(row, args(p), "abc1234")
            self.assertEqual(len(open(p).read().splitlines()[1].split("\t")), len(h.COLUMNS))


class Finished(unittest.TestCase):
    T = {"memos": {"time_limit_s": 180, "mem_cap_gb": 8}}

    def row(self, **kw):
        r = {"target": "memos", "wall_s": "50.0", "peak_tree_gb": "6.0", "converged": "yes"}
        r.update(kw)
        return r

    def test_finish_is_apart_from_the_detection(self):
        self.assertEqual(h.finished(self.row(detected="no"), self.T), "yes")
        self.assertEqual(h.finished(self.row(wall_s="181"), self.T), "no")
        self.assertEqual(h.finished(self.row(peak_tree_gb="8.1"), self.T), "no")
        self.assertEqual(h.finished(self.row(converged="no"), self.T), "no")
        self.assertEqual(h.finished(self.row(target="other"), self.T), "-")

if __name__ == "__main__":
    unittest.main()
