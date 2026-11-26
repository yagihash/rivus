#!/usr/bin/env python3
"""Tests of bench/maturity_ledger.py (python3 bench/test_maturity_ledger.py). Samples of the output of the tools only:
nothing is run."""
import os, sys, tempfile, unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import maturity_ledger as m

ZOO = """=== RUN   TestZooDart
    zoo_dart_test.go:291: zoo-dart: cells 139 / cases 525 | found 215 / 271 | @miss 56 (trace 44, silent 12, no rule 0) | fixed reported wrongly 3 | decoys reported wrongly 13 / 139 (with the trace container_element_imprecision 5, without 11) | carry map[] | manual 3 (0.6%) | run and annotation disagree 0 | statements not run 0
    zoo_dart_test.go:292: zoo-dart: parity marks: @na 4 [dyn/a dyn/b] | @todo 13 [call/x clos/y]
    zoo_dart_test.go:292: zoo-go: parity marks: @na 22 [dart/a] | @todo 45 [a b c] | @elsewhere 16 [dart/x dart/y] | elsewhere-miss 0 []
--- PASS: TestZooDart (8.87s)
=== RUN   TestZooRust
    zoo_rust_test.go:374: zoo-rust: cells 92 / cases 310 | found 108 / 136 | @miss 28 (trace 27, silent 1, no rule 0) | fixed reported wrongly 4
    zoo_rust_test.go:376: zoo-rust: parity marks: @na 0 [] | @todo 0 []
=== RUN   TestZooGo
    zoo_test.go:657: zoo-go: cells 89 / cases 288 | found 107 / 121 | @miss 14 (trace 4, silent 10, no rule 0) | fixed reported wrongly 0
    zoo_test.go:657: zoo-go: rivus: 186 flows found, 284 sinks quiet, 26 flows not found (@miss), 12 reported wrongly (@fp)
"""
GO_EDGES = """instrumented 693 functions (in a copy)
observed calls by class:
  ok           812
  no_edge      7
  no_callsite  0
"""
DART_EDGES = """call  callee_unresolved  3
call  no_edge            64
call  ok                 400
sdk   no_ref             2
distinct observed edges: 469, stacks: 100
"""


class Parse(unittest.TestCase):
    def test_todo(self):
        self.assertEqual(m.parse_todo(ZOO), {"dart": 13, "go": 45, "rust": 0})

    def test_marked_counts_marks_not_the_explanation(self):
        na = ("@todo(F/<family>/<shape>: PARITY: reason) is a form that Dart can have\n"
              "@na(F/dyn/a: x)\n@todo(F/clos/b: PARITY: later (maybe))\n")
        self.assertEqual(m.count_todo_marked(na), 1)
        self.assertEqual(m.count_todo_marked("@todo(F/a/x: PARITY: one)\n@todo(F/a/y: PARITY: two)\n@todo(F/a/y: PARITY: two)"), 2)
        self.assertEqual(m.count_todo_marked("@todo(F/a/x: later)"), 0)  # not a PARITY mark
        self.assertEqual(m.count_todo_marked(""), 0)

    def test_miss_reads_the_cells_line_only(self):
        self.assertEqual(m.parse_miss(ZOO), {"dart": 56, "rust": 28, "go": 14})

    def test_missing_lines_give_nothing(self):
        self.assertEqual(m.parse_todo("ok"), {})
        self.assertEqual(m.parse_miss(""), {})

    def test_denominators(self):
        self.assertEqual(m.parse_cells(ZOO), {"dart": 139, "rust": 92, "go": 89})
        self.assertEqual(m.parse_na(ZOO), {"dart": 4, "go": 22, "rust": 0})

    def test_elsewhere(self):
        # only the line that has the field gives a number: an empty cell is not a zero
        self.assertEqual(m.parse_elsewhere(ZOO), {"go": 16})
        self.assertEqual(m.parse_elsewhere("zoo-go: parity marks: @na 1 [a] | @todo 2 [b c]"), {})
        self.assertEqual(m.parse_todo(ZOO)["go"], 45)  # the fields before it are read as before

    def test_reach(self):
        log = "    zoo_rust_reach_test.go:157: zoo-rust-app: reach cells 9 | miss 0 | contrast ok\n"
        self.assertEqual(m.parse_reach(ZOO + log), (9, 0))
        self.assertEqual(m.parse_reach("zoo-rust-app: reach cells 12 | miss 3 | contrast ok"), (12, 3))

    def test_reach_empty_when_contrast_failed_or_no_line(self):
        self.assertIsNone(m.parse_reach("zoo-rust-app: reach cells 9 | miss 0 | contrast FAILED 1 / 2"))
        self.assertIsNone(m.parse_reach(ZOO))

    def test_calls_compared(self):
        self.assertEqual(m.parse_calls_go(GO_EDGES + "  id_unmapped  2\n  outside      50\n    outside: harness 40\n"), 812 + 7 + 0 + 2)
        self.assertIsNone(m.parse_calls_go("nothing"))
        self.assertEqual(m.parse_calls_dart(DART_EDGES), 3 + 64 + 400)
        self.assertIsNone(m.parse_calls_dart("Traceback"))

    def test_go_edges(self):
        self.assertEqual(m.parse_no_edge_go(GO_EDGES), 7)
        self.assertIsNone(m.parse_no_edge_go("observed-edges: the instrumented copy does not pass go vet"))

    def test_dart_edges(self):
        self.assertEqual(m.parse_no_edge_dart(DART_EDGES), 64)
        self.assertEqual(m.parse_no_edge_dart("call  ok  3\ndistinct observed edges: 3, stacks: 1\n"), 0)
        self.assertIsNone(m.parse_no_edge_dart("Traceback"))


class CallCheck(unittest.TestCase):
    def test_summary(self):
        out = "syntax_calls 3535\nfound 3530\ncall_unsupported 2\nallowed none\ncall_mismatch 0\n"
        self.assertEqual(m.parse_call_check(out), (0, 2))
        self.assertEqual(m.parse_call_check("Traceback"), (None, None))

    def test_dead(self):
        out = "syntax_calls 30\nfound 25\ncall_unsupported 0\ncall_dead 3\ndead {\"unreachable\": 3}\nallowed none\ncall_mismatch 1\n"
        self.assertEqual(m.parse_call_dead(out), 3)
        self.assertEqual(m.parse_call_check(out), (1, 0))
        self.assertIsNone(m.parse_call_dead("syntax_calls 3535\ncall_unsupported 2\ncall_mismatch 0\n"))  # the Dart tool has none


class Rows(unittest.TestCase):
    VALUES = {"go": {"cells": 89, "na": 22, "todo": 45, "todo_marked": 0, "miss": 14, "no_edge": 7, "calls_observed": 819, "call_mismatch": 0, "call_unsupported": 0, "call_dead": 3, "elsewhere": 16}, "dart": {"todo": 13, "todo_marked": 0, "miss": 56, "no_edge": None, "call_mismatch": 0, "call_unsupported": 2}, "rust": {"todo": 14, "todo_marked": 14, "miss": 28, "reach_total": 9, "reach_miss": 0}}

    def test_empty_is_empty_not_zero(self):
        rows = m.make_rows("2026-11-25", "abcd1234", self.VALUES)
        self.assertEqual(rows[0], ["2026-11-25", "abcd1234", "go", "89", "22", "45", "0", "14", "7", "819", "0", "0", "3", "16", "", ""])
        self.assertEqual(rows[1][8], "")
        self.assertEqual(rows[1][10:], ["0", "2", "", "", "", ""])  # a zero that was measured is written; an empty cell was not measured
        self.assertEqual(rows[2], ["2026-11-25", "abcd1234", "rust", "", "", "14", "14", "28", "", "", "", "", "", "", "9", "0"])

    def test_append_once(self):
        rows = m.make_rows("2026-11-25", "abcd1234", self.VALUES)
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "l.tsv")
            self.assertEqual(len(m.append(p, rows)), 3)
            self.assertEqual(m.append(p, rows), [])
            other = m.make_rows("2026-11-25", "ffff0000", self.VALUES)
            self.assertEqual(len(m.append(p, other)), 3)
            lines = open(p).read().splitlines()
            self.assertEqual(lines[0], "\t".join(m.COLUMNS))
            self.assertEqual(len(lines), 7)
            self.assertTrue(all(len(l.split("\t")) == len(m.COLUMNS) for l in lines))


class Committed(unittest.TestCase):
    def test_reach_columns_are_last_and_old_rows_are_empty(self):
        lines = [l.split("\t") for l in open(m.LEDGER).read().splitlines()]
        self.assertEqual(lines[0][-3:], ["elsewhere", "reach_total", "reach_miss"])
        self.assertEqual(lines[0][:14], m.COLUMNS[:14])
        self.assertTrue(all(r[14:] == ["", ""] for r in lines[1:]))
        self.assertTrue(all(r[2] in m.LANGS for r in lines[1:]))


    def test_every_row_has_every_column(self):
        # a column that is added is empty in the rows before it (not missing): the header and every row have the same width
        lines = open(m.LEDGER).read().splitlines()
        self.assertEqual(lines[0], "\t".join(m.COLUMNS))
        self.assertTrue(all(len(l.split("\t")) == len(m.COLUMNS) for l in lines))


if __name__ == "__main__":
    unittest.main()
