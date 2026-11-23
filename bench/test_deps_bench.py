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
        self.assertEqual(body, "time=120/180 mem=5.20/8 converged=yes finish=yes detected=yes quiet_after_fix=n/a")

    def test_each_criterion_alone_fails(self):
        self.assertFalse(d.judge(181, 180, 5, 8, True, True)[0])   # time
        self.assertFalse(d.judge(100, 180, 8.1, 8, True, True)[0])  # memory
        self.assertFalse(d.judge(100, 180, 5, 8, False, True)[0])  # convergence
        self.assertFalse(d.judge(100, 180, 5, 8, True, False)[0])  # detection

    def test_finish_and_detect_are_separate(self):
        body = d.judge(100, 180, 5, 8, True, False)[1]   # finished, not detected
        self.assertIn("finish=yes detected=no", body)
        body = d.judge(100, 180, 5, 8, False, True)[1]   # detected, did not finish
        self.assertIn("finish=no detected=yes", body)
        self.assertFalse(d.judge(100, 180, 5, 8, False, True)[0])

    def test_limits_are_inclusive(self):
        self.assertTrue(d.judge(180, 180, 8, 8, True, True)[0])

    def test_no_detection_is_not_a_failure(self):
        ok, body = d.judge(100, 180, 5, 8, True, None)
        self.assertTrue(ok)
        self.assertTrue(body.endswith("detected=n/a quiet_after_fix=n/a"))

    def test_missing_numbers_fail(self):
        self.assertFalse(d.judge(None, 180, 5, 8, True, True)[0])
        self.assertFalse(d.judge(10, 180, None, 8, True, True)[0])

    def test_line(self):
        ok, body = d.judge(67, 180, 8.8, 8, False, False)
        self.assertEqual(d.format_line("memos", True, ok, body), "FAIL memos time=67/180 mem=8.80/8 converged=no finish=no detected=no quiet_after_fix=n/a")
        self.assertTrue(d.format_line("dart_services", False, ok, body).startswith("INFO dart_services "))


class Detection(unittest.TestCase):
    def test_detected_at_prefix_only(self):
        self.assertTrue(d.detection(2))
        self.assertFalse(d.detection(0))
        self.assertFalse(d.detection(None))   # a run that did not converge

    def test_quiet_after_fix(self):
        self.assertTrue(d.quiet_after_fix(0))
        self.assertFalse(d.quiet_after_fix(1))
        self.assertFalse(d.quiet_after_fix(None))

    def test_quiet_after_fix_is_not_part_of_pass(self):
        self.assertTrue(d.judge(100, 180, 5, 8, True, True, None, False)[0])
        self.assertTrue(d.judge(100, 180, 5, 8, True, True, None, True)[0])
        self.assertTrue(d.judge(100, 180, 5, 8, True, True, None, False)[1].endswith("quiet_after_fix=no"))

    def test_count_category_counts_only(self):
        rep = {"findings": [{"category": "path_traversal"}, {"category": "xss"}, {"category": "path_traversal"}]}
        self.assertEqual(d.count_category(rep, "path_traversal"), 2)
        self.assertEqual(d.count_category({}, "path_traversal"), 0)


class Peak(unittest.TestCase):
    GB = 1024 ** 3

    def test_two_records_of_a_real_child(self):
        with tempfile.TemporaryDirectory() as w:
            m = d.run_watched([sys.executable, "-c", "x = bytearray(150 * 1024 * 1024); x[::4096] = b'a' * len(x[::4096]); import time; time.sleep(1.2)"],
                              w, dict(os.environ), 30, 8 * self.GB, os.path.join(w, "e"), os.path.join(w, "o"), interval=0.1)
        self.assertEqual((m["rc"], m["why"]), (0, ""))
        self.assertGreater(m["a"], 150 * 1024 * 1024)
        self.assertGreater(m["b"], 150 * 1024 * 1024)   # the tree is at least the main process
        self.assertTrue(m["big"])

    def test_watchdog_kills_over_the_cap(self):
        with tempfile.TemporaryDirectory() as w:
            m = d.run_watched([sys.executable, "-c", "x = bytearray(200 * 1024 * 1024); x[::4096] = b'a' * len(x[::4096]); import time; time.sleep(20)"],
                              w, dict(os.environ), 30, 100 * 1024 * 1024, os.path.join(w, "e"), os.path.join(w, "o"), interval=0.1)
        self.assertEqual(m["why"], "memory")
        self.assertGreater(m["b"], 100 * 1024 * 1024)

    def test_verdict_uses_the_tree_number(self):
        self.assertFalse(d.judge(100, 180, 9.0, 8, True, None)[0])


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
        self.assertTrue(body.endswith("detected=yes quiet_after_fix=n/a through_tp=4/9"))
        self.assertTrue(d.judge(100, 180, 5, 8, False, False, (None, 9))[1].endswith("detected=no quiet_after_fix=n/a through_tp=-/9"))


class Location(unittest.TestCase):
    F = "server/a.go"
    truth = [{"role": "source", "file": F, "line": 51, "fixed_line": 51, "category": "path_traversal"},
             {"role": "sink", "file": F, "line": 416, "fixed_line": 422, "category": "path_traversal"},
             {"role": "sink", "file": F, "line": 419, "fixed_line": 425, "category": "path_traversal"}]

    def rep(self, cat="path_traversal", src=51, snk=416, sfile=F):
        return {"findings": [{"category": cat, "source": {"position": {"file": sfile, "line": src}},
                              "sink": {"position": {"file": sfile, "line": snk}}}]}

    def test_finding_at_the_places_is_detected(self):
        n = d.location_counts(self.truth, self.rep(), "path_traversal")
        self.assertEqual(n, (1, 1, 2))
        self.assertTrue(d.detected_located(n[0]))

    def test_same_kind_at_other_places_is_not_detected(self):
        for r in (self.rep(snk=417), self.rep(src=52), self.rep(sfile="other.go")):
            self.assertFalse(d.detected_located(d.location_counts(self.truth, r, "path_traversal")[0]))

    def test_other_kind_at_the_places_is_not_detected(self):
        self.assertEqual(d.location_counts(self.truth, self.rep("ssrf"), "path_traversal")[0], 0)

    def test_fixed_side_table(self):
        self.assertEqual(d.location_counts(self.truth, self.rep(snk=422), "path_traversal", "fixed")[0], 1)
        self.assertEqual(d.location_counts(self.truth, self.rep(snk=422), "path_traversal", "prefix")[0], 0)

    def test_not_converged_is_not_detected(self):
        self.assertFalse(d.detected_located(None))
        self.assertFalse(d.detected_located(0))

    def test_memos_truth_file(self):
        t = json.load(open(d.TARGETS_FILE))["memos"]["detect"]
        self.assertEqual(t["mode"], "location")
        sites = json.load(open(os.path.join(d.ROOT, t["truth"])))["sites"]
        self.assertEqual(sorted(x["line"] for x in sites if x["role"] == "sink"), [416, 419, 426])
        self.assertEqual(sorted(x["fixed_line"] for x in sites if x["role"] == "sink"), [422, 425, 432])
        self.assertEqual([x["line"] for x in sites if x["role"] == "source"], [51])


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
