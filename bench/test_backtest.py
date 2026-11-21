#!/usr/bin/env python3
"""Tests of bench/backtest.py and bench/ledger.py (python3 bench/test_backtest.py). Synthetic input only; nothing is analysed."""
import json, os, subprocess, sys, unittest

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


if __name__ == "__main__":
    unittest.main()
