#!/usr/bin/env python3
"""Tests of the pass line of bench/deps_bench.py (python3 bench/test_deps_bench.py). Synthetic numbers only; nothing is fetched or analysed."""
import json, os, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import deps_bench as d


class Judge(unittest.TestCase):
    def test_all_four_met(self):
        ok, body = d.judge(120, 180, 5.2, 8, True, True)
        self.assertTrue(ok)
        self.assertEqual(body, "time=120/180 mem=5.20/8 converged=yes detected=yes")

    def test_each_criterion_alone_fails(self):
        self.assertFalse(d.judge(181, 180, 5, 8, True, True)[0])   # time
        self.assertFalse(d.judge(100, 180, 8.1, 8, True, True)[0])  # memory
        self.assertFalse(d.judge(100, 180, 5, 8, False, True)[0])  # convergence
        self.assertFalse(d.judge(100, 180, 5, 8, True, False)[0])  # detection

    def test_limits_are_inclusive(self):
        self.assertTrue(d.judge(180, 180, 8, 8, True, True)[0])

    def test_no_detection_is_not_a_failure(self):
        ok, body = d.judge(100, 180, 5, 8, True, None)
        self.assertTrue(ok)
        self.assertTrue(body.endswith("detected=n/a"))

    def test_missing_numbers_fail(self):
        self.assertFalse(d.judge(None, 180, 5, 8, True, True)[0])
        self.assertFalse(d.judge(10, 180, None, 8, True, True)[0])

    def test_line(self):
        ok, body = d.judge(67, 180, 8.8, 8, False, False)
        self.assertEqual(d.format_line("memos", True, ok, body), "FAIL memos time=67/180 mem=8.80/8 converged=no detected=no")
        self.assertTrue(d.format_line("dart_services", False, ok, body).startswith("INFO dart_services "))


class Detection(unittest.TestCase):
    def test_more_before_than_after(self):
        self.assertTrue(d.detection(2, 0))
        self.assertFalse(d.detection(0, 0))
        self.assertFalse(d.detection(1, 1))
        self.assertFalse(d.detection(None, 0))   # a run that did not converge
        self.assertFalse(d.detection(3, None))

    def test_count_category_counts_only(self):
        rep = {"findings": [{"category": "path_traversal"}, {"category": "xss"}, {"category": "path_traversal"}]}
        self.assertEqual(d.count_category(rep, "path_traversal"), 2)
        self.assertEqual(d.count_category({}, "path_traversal"), 0)


class Through(unittest.TestCase):
    truth = [{"file": "a.dart", "line": 1, "label": "vuln", "category": "path_traversal"},
             {"file": "a.dart", "line": 2, "label": "vuln", "category": "path_traversal"},
             {"file": "a.dart", "line": 3, "label": "safe", "category": "path_traversal"}]

    def rep(self, cat, line):
        return {"findings": [{"category": cat, "paths": [{"steps": [{"desc": "call f", "position": {"file": "a.dart", "line": line}}]}]}]}

    def test_counts_labelled_positions(self):
        self.assertEqual(d.through_counts(self.truth, self.rep("path_traversal", 2)), (1, 2))
        self.assertEqual(d.through_counts(self.truth, {}), (0, 2))

    def test_kind_mismatch_not_counted(self):
        self.assertEqual(d.through_counts(self.truth, self.rep("ssrf", 1)), (0, 2))

    def test_non_call_step_not_counted(self):
        r = self.rep("path_traversal", 1)
        r["findings"][0]["paths"][0]["steps"][0]["desc"] = "assign"
        self.assertEqual(d.through_counts(self.truth, r)[0], 0)

    def test_per_vulnerability(self):
        self.assertTrue(d.detected_through(1))
        self.assertFalse(d.detected_through(0))
        self.assertFalse(d.detected_through(None))

    def test_line_has_both_numbers(self):
        ok, body = d.judge(100, 180, 5, 8, True, True, (4, 9))
        self.assertTrue(ok)
        self.assertTrue(body.endswith("detected=yes through_tp=4/9"))
        self.assertTrue(d.judge(100, 180, 5, 8, False, False, (None, 9))[1].endswith("detected=no through_tp=-/9"))


class Targets(unittest.TestCase):
    def test_pins_and_limits(self):
        t = json.load(open(d.TARGETS_FILE))
        self.assertEqual(t["memos"]["commit"], "00fcd170281a75fc0e95ec4f2f54f43d32fe81bd")
        self.assertEqual(t["memos"]["detect"]["prefix_commit"], "1d7efb158045924ab87f58d567bd79cd92edf6af")
        self.assertEqual(t["memos"]["detect"]["fixed_commit"], "5f57f48673e2054f404b2c5b497a8eaa3690591d")
        self.assertEqual((t["memos"]["time_limit_s"], t["memos"]["mem_cap_gb"], t["memos"]["args"]), (180, 8, []))
        self.assertEqual(t["dart_services"]["commit"], "ad668d137e631beeaef3e263054a45696e8d4499")
        self.assertFalse(t["dart_services"]["decided"])

    def test_cocoon_target(self):
        t = json.load(open(d.TARGETS_FILE))["cocoon"]
        self.assertEqual(t["commit"], "ae9eb1beaf765d644cc85dde38def030c80b4b7d")
        self.assertEqual((t["subdir"], t["decided"], t["detect"], t["time_limit_s"], t["mem_cap_gb"]), ("app_dart", True, None, 180, 8))
        self.assertEqual(d.expand_args(t["args"], "/c/app_dart", t["subdir"]), ["-allow-deps-under", "/c/packages"])
        self.assertEqual(d.expand_args(["-x", "{root}/p"], "/c/a/b", "a/b"), ["-x", "/c/p"])
        self.assertEqual(d.expand_args([], "/c", ""), [])

    def test_cache_outside_repo(self):
        os.environ["BENCH_CACHE"] = os.path.join(d.ROOT, "cache")
        try:
            with self.assertRaises(SystemExit):
                d.cache_dir()
        finally:
            del os.environ["BENCH_CACHE"]

    def test_flutter_members_removed_from_the_copy(self):
        with tempfile.TemporaryDirectory() as r:
            for m, body in (("pkgs/a", "name: a\n"), ("pkgs/b", "name: b\ndependencies:\n  flutter:\n    sdk: flutter\n")):
                os.makedirs(os.path.join(r, m))
                open(os.path.join(r, m, "pubspec.yaml"), "w").write(body)
            open(os.path.join(r, "pubspec.yaml"), "w").write("name: root\nworkspace:\n  - pkgs/a\n  - pkgs/b\nenvironment:\n  sdk: ^3.0.0\n")
            d.remove_flutter_members(r)
            s = open(os.path.join(r, "pubspec.yaml")).read()
            self.assertIn("pkgs/a", s)
            self.assertNotIn("pkgs/b", s)
            self.assertIn("environment:", s)


if __name__ == "__main__":
    unittest.main()
