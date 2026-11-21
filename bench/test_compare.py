#!/usr/bin/env python3
"""Unit tests of bench/compare.py report (python3 bench/test_compare.py): the exit status of a surge of added findings,
a head that did not converge, and a clean pair. Synthetic results only; nothing is analysed."""
import json, os, subprocess, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def finding(i):
    return {"source": {"position": {"file": "a.go", "line": i}, "func": "s"}, "sink": {"position": {"file": "b.go", "line": i}, "symbol": "x"},
            "category": "c", "reachability": "r", "paths": [{"steps": [{"func": "s"}]}]}


def write(d, bench, side, n_findings, **stats):
    st = {"functions": 100, "converged": True, "passes": 1, "reanalyses": 1}
    st.update(stats)
    json.dump({"findings": [finding(i) for i in range(n_findings)], "uncertainties": [], "stats": st}, open("%s/%s.%s.json" % (d, bench, side), "w"))
    json.dump({"rc": 0, "wall_s": 1, "max_rss_mb": 1}, open("%s/%s.%s.meta.json" % (d, bench, side), "w"))


def report(base_n, head_n, **head_stats):
    with tempfile.TemporaryDirectory() as d:
        write(d, "b", "base", base_n)
        write(d, "b", "head", head_n, **head_stats)
        r = subprocess.run([sys.executable, os.path.join(HERE, "compare.py"), "report", "-results", d, "-base", "A", "-head", "B",
                            "-out", d + "/out", "-benches", "b"], capture_output=True, text=True)
        return r.returncode, open(d + "/out/summary.md").read()


class Report(unittest.TestCase):
    def test_clean(self):
        self.assertEqual(report(50, 50)[0], 0)

    def test_small_addition_is_fine(self):
        self.assertEqual(report(50, 53)[0], 0)  # 3 added

    def test_surge_of_additions_is_exit_2(self):
        rc, text = report(77, 124)  # +47 (61%), as in the gogs run of C-7
        self.assertEqual(rc, 2)
        self.assertIn("NOISE: b +47 (61%): classification needed", text)

    def test_many_but_under_20_percent(self):
        self.assertEqual(report(100, 119)[0], 0)  # +19: 19% only

    def test_over_20_percent_but_5_or_fewer(self):
        self.assertEqual(report(10, 15)[0], 0)  # +5 (50%): not more than 5

    def test_head_not_converged_is_exit_2(self):
        self.assertEqual(report(50, 50, converged=False)[0], 2)

    def test_removal_is_still_exit_1(self):
        self.assertEqual(report(50, 40)[0], 1)


if __name__ == "__main__":
    unittest.main()
