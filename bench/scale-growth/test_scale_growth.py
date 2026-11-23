#!/usr/bin/env python3
"""Tests of bench/scale-growth/scale_growth.py (python3 bench/scale-growth/test_scale_growth.py). Fake output only; nothing is run."""
import os, sys, unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import scale_growth as s

GC = "gc %d @1.0s 2%%: 0.03+2.8+0.01 ms clock, 0.3+0/7/17+0.1 ms cpu, %d->%d->%d MB, 90 MB goal, 0 MB stacks, 2 MB globals, 12 P%s"
FAKE = "\n".join([
    GC % (1, 40, 41, 30, ""),
    GC % (2, 60, 60, 55, " (forced)"),
    "rivus: ir         619ms  (heap 55 MB)",
    "rivus: sites      12ms  (heap 55 MB)",
    GC % (3, 70, 72, 60, " (forced)"),
    "rivus: graph      43ms  (heap 27 MB)",
    GC % (4, 100, 110, 400, ""),
    GC % (5, 500, 520, 900, ""),
    GC % (6, 800, 810, 700, ""),
    GC % (7, 800, 800, 700, " (forced)"),
    "rivus: taint      202ms  (heap 11 MB)",
    "rivus: 107 distinct strings in the elements",
])


class Parse(unittest.TestCase):
    def test_stages(self):
        r = s.parse_stderr(FAKE)
        self.assertEqual(r["load_mb"], 27)      # the phase before taint (graph), after a forced collection
        self.assertEqual(r["summary_mb"], 900)  # the largest marked value between graph and taint; the forced ones do not count
        self.assertEqual(r["after_mb"], 11)
        self.assertEqual(r["phases"], {"ir": 55, "sites": 55, "graph": 27, "taint": 11})
        self.assertEqual(r["gcs"], 7)

    def test_optional_phase_before_taint(self):
        text = FAKE.replace("rivus: taint", "rivus: graphdb    9ms  (heap 30 MB)\n" + GC % (8, 90, 90, 950, "") + "\nrivus: taint")
        r = s.parse_stderr(text)
        self.assertEqual(r["load_mb"], 30)
        self.assertEqual(r["summary_mb"], 950)  # only the window after graphdb

    def test_no_taint_phase(self):
        r = s.parse_stderr("rivus: ir  1ms  (heap 5 MB)\n")
        self.assertIsNone(r["load_mb"])
        self.assertIsNone(r["summary_mb"])

    def test_other_lines_ignored(self):
        self.assertEqual(s.parse_stderr("hello\nscavenge 1\n")["gcs"], 0)


class Growth(unittest.TestCase):
    def test_ratio_and_exponent(self):
        rows = [{"scale": 1, "load_mb": 100, "summary_mb": 200}, {"scale": 2, "load_mb": 200, "summary_mb": 800},
                {"scale": 4, "load_mb": 400, "summary_mb": None}]
        g = s.growth(rows, ("load_mb", "summary_mb"))
        self.assertEqual(g[0]["load_mb"], {"ratio": 2.0, "exponent": 1.0})
        self.assertEqual(g[0]["summary_mb"], {"ratio": 4.0, "exponent": 2.0})
        self.assertEqual((g[1]["from"], g[1]["to"]), (2, 4))
        self.assertIsNone(g[1]["summary_mb"])  # a missing value gives no ratio

    def test_uneven_scales(self):
        g = s.growth([{"scale": 1, "x": 10}, {"scale": 4, "x": 40}], ("x",))
        self.assertEqual(g[0]["x"], {"ratio": 4.0, "exponent": 1.0})

    def test_table_prints(self):
        rows = [{"scale": 1, "load_mb": 10, "summary_mb": 20, "functions": 5, "summary_elems": 1, "summary_steps": 2, "wall_s": 1.5,
                 "cpu_s": 2.0, "memory_peak_bytes": 1 << 30, "gc_cpu_fraction": 0.05}]
        self.assertIn("1024", s.table(rows, []))


class Profiles(unittest.TestCase):
    def test_scale_moves_the_knobs(self):
        self.assertIn("80", s.PROFILES["mixed"](2))
        self.assertIn("4000", s.PROFILES["mixed"](2))
        self.assertNotIn("-funcs", s.PROFILES["ops"](2))


if __name__ == "__main__":
    unittest.main()
