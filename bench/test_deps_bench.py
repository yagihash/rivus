#!/usr/bin/env python3
"""Tests of the pass line of bench/deps_bench.py (python3 bench/test_deps_bench.py). Synthetic numbers only; nothing is fetched or analysed."""
import json, os, subprocess, sys, tempfile, unittest

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

    def test_coarsened_components_must_be_the_expected_number(self):
        # a target that is not meant to be hit has 0; a target that is hit without being said fails the line
        ok, body = d.judge(100, 180, 5, 8, True, True, coarsened=0)
        self.assertTrue(ok)
        self.assertTrue(body.endswith("coarsened=0/0"), body)
        self.assertFalse(d.judge(100, 180, 5, 8, True, True, coarsened=1)[0])
        self.assertTrue(d.judge(100, 180, 5, 8, True, True, coarsened=1, expect_coarsened=1)[0])
        self.assertFalse(d.judge(100, 180, 5, 8, True, True, coarsened=0, expect_coarsened=1)[0])
        # without the number (an older binary, or no report) it is not part of the verdict
        self.assertTrue(d.judge(100, 180, 5, 8, True, True)[0])

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

    def test_sink_step_counted(self):
        r = self.rep("path_traversal", 1)
        r["findings"][0]["paths"][0]["steps"][0]["desc"] = "sink: dart:io#Link.create"
        self.assertEqual(d.through_counts(self.truth, r), (1, 2))

    def test_sink_step_of_another_category_not_counted(self):
        r = self.rep("ssrf", 1)
        r["findings"][0]["paths"][0]["steps"][0]["desc"] = "sink: dart:io#Link.create"
        self.assertEqual(d.through_counts(self.truth, r), (0, 2))

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


def finding(src, sink_file, sink_line, cat, funcs):
    return {"category": cat, "source": {"func": src}, "sink": {"position": {"file": sink_file, "line": sink_line}},
            "paths": [{"steps": [{"func": f, "desc": "x"} for f in funcs]}]}


class Score(unittest.TestCase):
    """detect.mode score (Gitea): per published vulnerability, by place; the count n/N only."""
    TRUTH = [
        {"source_func": "a.A", "via": ["b.B"], "sink_file": "s.go", "sink_line": 1, "label": "vuln", "category": "ssrf", "vuln": 1},
        {"source_func": "a.C", "sink_file": "t.go", "sink_line": 2, "label": "vuln", "category": "path_traversal", "vuln": 2},
        {"source_func": "a.D", "sink_file": "u.go", "sink_line": 3, "label": "vuln", "category": "ssrf", "vuln": 3},
        {"source_func": "a.E", "sink_file": "u.go", "sink_line": 3, "label": "vuln", "category": "ssrf", "vuln": 3},
        {"source_func": "a.F", "sink_file": "v.go", "sink_line": 4, "label": "safe", "category": "ssrf", "vuln": 9},
    ]

    def test_counts_per_vulnerability(self):
        rep = {"findings": [finding("x/a.A", "s.go", 1, "ssrf", ["b.B"]), finding("a.E", "u.go", 3, "ssrf", [])]}
        self.assertEqual(d.score_counts(self.TRUTH, rep), (2, 3))  # vuln 3 is hit by one of its two entries; the safe entry is not counted

    def test_via_must_be_on_a_path(self):
        rep = {"findings": [finding("a.A", "s.go", 1, "ssrf", ["other"])]}
        self.assertEqual(d.score_counts(self.TRUTH, rep), (0, 3))

    def test_category_and_sink_must_match(self):
        rep = {"findings": [finding("a.C", "t.go", 2, "ssrf", []), finding("a.D", "u.go", 99, "ssrf", [])]}
        self.assertEqual(d.score_counts(self.TRUTH, rep), (0, 3))

    def test_the_line_carries_n_over_N_only(self):
        ok, body = d.judge(500, 600, 6, 8, True, None, score=(0, 3))
        self.assertTrue(ok)  # not in the verdict
        self.assertEqual(body, "time=500/600 mem=6.00/8 converged=yes finish=yes detected=0/3 quiet_after_fix=n/a")
        self.assertFalse(d.judge(601, 600, 6, 8, True, None, score=(3, 3))[0])
        self.assertIn("detected=-/3", d.judge(500, 600, 6, 8, False, None, score=(None, 3))[1])

    def test_gitea_truth_file_has_three_vulnerabilities(self):
        truth = json.load(open(os.path.join(d.ROOT, "bench/gitea-cve-truth.json")))["sites"]
        self.assertEqual(d.score_counts(truth, {"findings": []}), (0, 3))


class PinLock(unittest.TestCase):
    def test_lock_is_copied_to_the_root_of_the_tree(self):
        with tempfile.TemporaryDirectory() as root:
            d.pin_lock({"lock": "bench/dart-services/pubspec.lock"}, root)
            want = open(os.path.join(d.ROOT, "bench/dart-services/pubspec.lock")).read()
            self.assertEqual(open(os.path.join(root, "pubspec.lock")).read(), want)

    def test_an_existing_lock_in_the_tree_is_replaced(self):
        with tempfile.TemporaryDirectory() as root:
            open(os.path.join(root, "pubspec.lock"), "w").write("other")
            d.pin_lock({"lock": "bench/dart-services/pubspec.lock"}, root)
            self.assertNotEqual(open(os.path.join(root, "pubspec.lock")).read(), "other")

    def test_no_lock_field_writes_nothing(self):
        with tempfile.TemporaryDirectory() as root:
            d.pin_lock({}, root)
            self.assertEqual(os.listdir(root), [])

    def test_only_dart_services_has_a_lock(self):
        t = json.load(open(d.TARGETS_FILE))
        self.assertEqual([n for n, v in t.items() if isinstance(v, dict) and v.get("lock")], ["dart_services"])
        self.assertTrue(os.path.exists(os.path.join(d.ROOT, t["dart_services"]["lock"])))


class Targets(unittest.TestCase):
    def test_pass_line_a_and_c(self):
        t = json.load(open(d.TARGETS_FILE))
        a, c = t["memos-4gib"], t["gitea"]
        self.assertEqual((a["args"], a["time_limit_s"], a["mem_cap_gb"], a["detect"]["mode"]), (["-max-memory", "4GiB"], 180, 4, "location"))
        self.assertEqual((c["args"], c["time_limit_s"], c["mem_cap_gb"], c["detect"]["mode"]), (["-no-project-rules"], 600, 8, "score"))
        self.assertEqual(c["commit"], "95ba37d9af58db7d9163f9e92e07b5eb4f792bbc")
        truth = json.load(open(os.path.join(d.ROOT, c["detect"]["truth"])))["sites"]
        self.assertTrue(all("vuln" in x for x in truth if x["label"] == "vuln"))

    def test_makefile_default_targets_and_gitea_by_choice(self):
        mk = open(os.path.join(d.ROOT, "Makefile")).read()
        line = [l for l in mk.splitlines() if l.startswith("\tpython3 bench/deps_bench.py") and "TARGETS" in l][0]
        self.assertIn("memos-4gib cocoon", line)
        self.assertNotIn("memos memos-4gib", line)   # the memos 8 GB row left the pass line (2026-11-25)
        self.assertNotIn("gitea", line)

    def test_a_carries_the_detection_of_the_removed_b_row(self):
        # B (memos, 8 GB) was removed from the pass line on 2026-11-25: A (memos-4gib) holds the memory and the time (stricter)
        # and the same detect, and the detection is part of A's verdict
        t = json.load(open(d.TARGETS_FILE))
        a, b = t["memos-4gib"], t["memos"]
        self.assertEqual((a["repo"], a["commit"], a["subdir"]), (b["repo"], b["commit"], b["subdir"]))
        self.assertLess(a["mem_cap_gb"], b["mem_cap_gb"])
        self.assertLessEqual(a["time_limit_s"], b["time_limit_s"])
        self.assertEqual(a["detect"], b["detect"])
        for k in ("id", "mode", "truth", "category", "prefix_commit", "fixed_commit"):
            self.assertTrue(a["detect"].get(k), k)
        self.assertTrue(a["decided"])
        cap, lim = a["mem_cap_gb"], a["time_limit_s"]
        self.assertTrue(d.judge(100, lim, cap - 0.5, cap, True, True)[0])
        self.assertFalse(d.judge(100, lim, cap - 0.5, cap, True, False)[0])   # not detected: A fails
        self.assertFalse(d.judge(100, lim, cap - 0.5, cap, False, True)[0])   # detected, not converged: A fails
        self.assertTrue(d.judge(100, lim, cap - 0.5, cap, True, True, None, False)[0])  # quiet_after_fix is not in the verdict

    def test_rivus_is_not_given_a_timeout(self):
        # the time limit is the watchdog's; a rivus that stopped itself at the limit would look like "not converged"
        t = json.load(open(d.TARGETS_FILE))
        for name in ("memos", "memos-4gib", "gitea"):
            self.assertNotIn("-timeout", t[name]["args"])
        src = open(os.path.join(HERE, "deps_bench.py")).read()
        self.assertNotIn('"-timeout", "%ds"', src.split("def analyze_once")[1].split("def passline")[0])

    def test_pins_and_limits(self):
        t = json.load(open(d.TARGETS_FILE))
        self.assertEqual(t["memos"]["commit"], "00fcd170281a75fc0e95ec4f2f54f43d32fe81bd")
        self.assertEqual(t["memos"]["detect"]["prefix_commit"], "1d7efb158045924ab87f58d567bd79cd92edf6af")
        self.assertEqual(t["memos"]["detect"]["fixed_commit"], "5f57f48673e2054f404b2c5b497a8eaa3690591d")
        self.assertEqual((t["memos"]["time_limit_s"], t["memos"]["mem_cap_gb"], t["memos"]["args"]), (180, 8, []))
        self.assertEqual(t["dart_services"]["commit"], "ad668d137e631beeaef3e263054a45696e8d4499")
        ds = t["dart_services"]
        self.assertEqual((ds["decided"], ds["detect"], ds["time_limit_s"], ds["mem_cap_gb"], ds["args"]), (True, None, 600, 8, []))
        self.assertTrue(d.format_line("dart_services", ds["decided"], False, "x").startswith("FAIL dart_services "))

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


class RustRoad(unittest.TestCase):
    """The Rust row (shlex): the refusal of a configuration, the check of the isolation, the lock, the line. No network, no fetch, no build."""
    class Done:
        def __init__(self, rc, err=""):
            self.returncode, self.stderr = rc, err

    def tree(self, files):
        r = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(r, ignore_errors=True))
        for rel, body in files.items():
            os.makedirs(os.path.dirname(os.path.join(r, rel)), exist_ok=True)
            open(os.path.join(r, rel), "w").write(body)
        return r

    def test_target_row(self):
        t = json.load(open(d.TARGETS_FILE))["shlex"]
        self.assertEqual((t["lang"], t["commit"], t["decided"], t["detect"], t["rust_minimal_lock"]), ("rust", "4a0724b0b62ef715467875b040a890ce75a8a829", False, None, True))
        self.assertEqual(t["repo"], "https://github.com/comex/rust-shlex")

    def test_not_in_the_default_targets(self):
        mk = open(os.path.join(d.ROOT, "Makefile")).read()
        line = [l for l in mk.splitlines() if "deps_bench.py" in l and "TARGETS" in l and not l.startswith("#")]
        self.assertTrue(line)
        self.assertTrue(all("shlex" not in l for l in line))

    def test_cargo_config_is_refused(self):
        r = self.tree({"Cargo.toml": "[package]\nname='a'\nversion='1.0.0'\n", ".cargo/config.toml": "[build]\nrustc-wrapper = \"/tmp/x\"\n"})
        self.assertEqual([p for p, _ in d.rust_refusals(r)], [os.path.join(".cargo", "config.toml")])
        with self.assertRaises(SystemExit) as c:
            d.refuse_rust("shlex", r)
        self.assertIn("rustc-wrapper", str(c.exception))  # what the file says is printed
        self.assertIn("nothing was built or run", str(c.exception))

    def test_old_config_name_and_toolchain_files_are_refused(self):
        for name in (".cargo/config", "rust-toolchain", "rust-toolchain.toml", "sub/.cargo/config.toml", "sub/rust-toolchain.toml"):
            r = self.tree({"Cargo.toml": "", name: "x"})
            self.assertEqual(len(d.rust_refusals(r)), 1, name)

    def test_clean_tree_and_other_files_pass(self):
        r = self.tree({"Cargo.toml": "", "src/lib.rs": "", "config.toml": "x", "docs/config": "x", ".git/rust-toolchain": "x"})
        self.assertEqual(d.rust_refusals(r), [])
        d.refuse_rust("shlex", r)

    def test_prepare_refuses_before_any_copy_or_tool(self):
        r = self.tree({"Cargo.toml": "", "rust-toolchain.toml": "[toolchain]\nchannel='stable'\n"})
        w = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(w, ignore_errors=True))
        with self.assertRaises(SystemExit) as c:
            d.prepare("shlex", {"lang": "rust"}, r, w, dict(os.environ))
        self.assertIn("channel", str(c.exception))
        self.assertEqual(os.listdir(w), [])

    def test_isolation_denied_is_ok(self):
        calls = []
        run = lambda cmd, **k: (calls.append(cmd), self.Done(1, "touch: /x: Operation not permitted\n"))[1]
        with tempfile.TemporaryDirectory() as w:
            self.assertEqual(d.sandbox_denies_write(w, run), (True, ""))
        self.assertEqual(calls[0][:2], ["/usr/bin/sandbox-exec", "-p"])
        self.assertIn("(deny file-write*)", calls[0][2])

    def test_write_not_denied_fails(self):
        def run(cmd, **k):
            open(cmd[-1], "w").close()  # the write happened
            return self.Done(0)
        with tempfile.TemporaryDirectory() as w:
            ok, why = d.sandbox_denies_write(w, run)
            self.assertFalse(ok)
            self.assertIn("NOT denied", why)
            self.assertEqual(os.listdir(w), [])  # the probe file is removed

    def test_exit_zero_without_a_file_fails(self):
        with tempfile.TemporaryDirectory() as w:
            self.assertFalse(d.sandbox_denies_write(w, lambda cmd, **k: self.Done(0))[0])

    def test_sandbox_that_did_not_run_the_probe_fails(self):
        with tempfile.TemporaryDirectory() as w:
            ok, why = d.sandbox_denies_write(w, lambda cmd, **k: self.Done(71, "sandbox_apply: Operation not permitted\n"))
            self.assertFalse(ok)
            self.assertIn("did not run the probe", why)

    def test_no_sandbox_exec_fails(self):
        def run(cmd, **k):
            raise FileNotFoundError("sandbox-exec")
        with tempfile.TemporaryDirectory() as w:
            self.assertFalse(d.sandbox_denies_write(w, run)[0])

    def test_report_isolation_mode(self):
        self.assertTrue(d.isolation_ok({"summary": {"isolation": {"mode": "sandbox"}}})[0])
        self.assertFalse(d.isolation_ok({"summary": {"isolation": {"mode": "host"}}})[0])
        self.assertFalse(d.isolation_ok({"summary": {}})[0])
        self.assertFalse(d.isolation_ok(None)[0])

    def test_uncertainty_counts_and_the_line(self):
        rep = {"uncertainties": [{"kind": "unmodeled_library_call"}] * 13}
        n = d.uncertainty_counts(rep)
        self.assertEqual(n, {"unmodeled_library_call": 13, "unsupported_construct": 0, "unresolved_callee": 0})
        self.assertEqual(d.format_uncertainties(n), "uncertainties unmodeled_library_call=13 unresolved_callee=0 unsupported_construct=0")
        self.assertEqual(d.uncertainty_counts(None), {"unsupported_construct": 0, "unresolved_callee": 0})

    def test_the_line_is_info(self):
        ok, body = d.judge(0.8, 180, 0.1, 8, True, None)
        self.assertEqual(d.format_line("shlex", False, ok, body).split()[:2], ["INFO", "shlex"])
        self.assertIn("detected=n/a", body)

    TOML = '[package]\nname = "shlex"\nversion = "1.3.0"\n\n[features]\nstd = []\ndefault = ["std"]\n'

    def test_minimal_lock_is_only_for_no_dependency(self):
        self.assertTrue(d.rust_no_dependencies(self.TOML))
        for extra in ('[dependencies]\nlibc = "0.2"\n', '[dev-dependencies]\nx = "1"\n', '[build-dependencies]\nx = "1"\n',
                      '[target.\'cfg(unix)\'.dependencies]\nx = "1"\n', "[workspace]\nmembers = []\n"):
            self.assertFalse(d.rust_no_dependencies(self.TOML + extra), extra)
        self.assertEqual(d.minimal_lock(self.TOML),
                         '# This file is automatically @generated by Cargo.\n# It is not intended for manual editing.\nversion = 4\n\n[[package]]\nname = "shlex"\nversion = "1.3.0"\n')

    def test_prepare_writes_the_lock_in_the_copy_only(self):
        r = self.tree({"Cargo.toml": self.TOML, "src/lib.rs": ""})
        w = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(w, ignore_errors=True))
        tc = self.tree({"bin/rustc": ""})
        ex = os.path.join(tc, "ex")
        open(ex, "w").close()
        os.chmod(ex, 0o755)
        old = {k: os.environ.get(k) for k in ("RIVUS_RUST_TOOLCHAIN_DIR", "RIVUS_RUST_EXTRACTOR")}
        os.environ["RIVUS_RUST_TOOLCHAIN_DIR"], os.environ["RIVUS_RUST_EXTRACTOR"] = tc, ex
        try:
            src, env, err = d.prepare("shlex", {"lang": "rust", "rust_minimal_lock": True}, r, w, dict(os.environ))
        finally:
            for k, v in old.items():
                os.environ.pop(k, None) if v is None else os.environ.__setitem__(k, v)
        self.assertIsNone(err)
        self.assertTrue(os.path.exists(os.path.join(src, "Cargo.lock")))
        self.assertFalse(os.path.exists(os.path.join(r, "Cargo.lock")))
        self.assertEqual((env["RIVUS_RUST_TOOLCHAIN_DIR"], env["RIVUS_RUST_EXTRACTOR"]), (tc, ex))

    def test_rustypaste_row(self):
        t = json.load(open(d.TARGETS_FILE))["rustypaste"]
        self.assertEqual((t["lang"], t["commit"], t["decided"], t["detect"], t["repo"]), ("rust", "00292076e47b5f500906a97a1b5c17665270d907", False, None, "https://github.com/orhun/rustypaste"))
        self.assertNotIn("rust_minimal_lock", t)  # the committed Cargo.lock is used
        self.assertEqual(t["args"], ["-timeout", "600s"])
        self.assertEqual(t["mem_cap_gb"], 8)
        self.assertNotIn("rustypaste", d.BENCHES)  # not in the default targets
        mk = open(os.path.join(d.ROOT, "Makefile")).read()
        self.assertNotIn("rustypaste", mk)
        self.assertEqual(d.format_line("rustypaste", False, True, "x").split()[:2], ["INFO", "rustypaste"])

    def test_committed_lock_is_used_and_never_generated(self):
        w = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(w, ignore_errors=True))
        tc = self.tree({"bin/rustc": ""})
        ex = os.path.join(tc, "ex")
        open(ex, "w").close()
        os.chmod(ex, 0o755)
        old = {k: os.environ.get(k) for k in ("RIVUS_RUST_TOOLCHAIN_DIR", "RIVUS_RUST_EXTRACTOR")}
        os.environ["RIVUS_RUST_TOOLCHAIN_DIR"], os.environ["RIVUS_RUST_EXTRACTOR"] = tc, ex
        try:
            lock = '# committed\nversion = 4\n\n[[package]]\nname = "a"\nversion = "1.0.0"\n'
            r = self.tree({"Cargo.toml": '[package]\nname="a"\nversion="1.0.0"\n[dependencies]\nlibc = "0.2"\n', "Cargo.lock": lock})
            src, _, err = d.prepare("rustypaste", {"lang": "rust"}, r, w, dict(os.environ))
            self.assertIsNone(err)
            self.assertEqual(open(os.path.join(src, "Cargo.lock")).read(), lock)  # as committed, not rewritten
            r2 = self.tree({"Cargo.toml": '[package]\nname="b"\nversion="1.0.0"\n[dependencies]\nlibc = "0.2"\n'})
            src2, _, err2 = d.prepare("rustypaste", {"lang": "rust"}, r2, w, dict(os.environ))
            self.assertIn("no committed Cargo.lock", err2)
            self.assertFalse(os.path.exists(os.path.join(r2, "Cargo.lock")))  # nothing was generated
        finally:
            for k, v in old.items():
                os.environ.pop(k, None) if v is None else os.environ.__setitem__(k, v)

    def test_stale_extractor_is_refused(self):
        r = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(r, ignore_errors=True))
        for c in (["git", "init", "-q"], ["git", "config", "user.email", "a@b"], ["git", "config", "user.name", "a"], ["git", "config", "commit.gpgsign", "false"]):
            subprocess.run(c, cwd=r, check=True)
        os.makedirs(os.path.join(r, "extractor/rust"))
        open(os.path.join(r, "extractor/rust/a.rs"), "w").write("x")
        subprocess.run(["git", "add", "-A"], cwd=r, check=True)
        subprocess.run(["git", "commit", "-q", "-m", "x"], cwd=r, check=True)
        ex = os.path.join(r, "ex")
        open(ex, "w").close()
        os.utime(ex, (1, 1))  # older than the commit
        self.assertIn("older than", d.extractor_stale(ex, r))
        os.utime(ex, None)
        self.assertEqual(d.extractor_stale(ex, r), "")

    def test_failed_run_is_not_an_isolation_failure_and_adds_no_row(self):
        tmp = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(tmp, ignore_errors=True))
        t = {"lang": "rust", "repo": "x", "commit": "c", "args": [], "time_limit_s": 100, "mem_cap_gb": 8, "decided": False, "detect": None, "local": "bench/go-curated"}
        saved = (d.prepare, d.analyze_once)
        d.prepare = lambda name, t, tree, work, env: (tree, env, None)
        d.analyze_once = lambda *a, **k: {"wall": 91.0, "peak": 6 * 1024 ** 3, "converged": False, "why": "", "report": None, "peak_how": "x", "peak_a": 0, "cpu": 1.0, "big": "", "big_rss": 0, "at_s": 0, "rc": 1, "load": 1.0, "note": "n"}
        try:
            line, row = d.passline_run("rustypaste", t, "rivus", tmp, tmp, 100, 8, 1, tmp)
            t2 = dict(t)
            d.analyze_once = lambda *a, **k: {"wall": 1.0, "peak": 9 * 1024 ** 3, "converged": False, "why": "memory", "report": None, "peak_how": "x", "peak_a": 0, "cpu": 1.0, "big": "", "big_rss": 0, "at_s": 0, "rc": -9, "load": 1.0, "note": "n"}
            line2, row2 = d.passline_run("rustypaste", t2, "rivus", tmp, tmp, 100, 8, 1, tmp)
        finally:
            d.prepare, d.analyze_once = saved
        self.assertIsNone(row)
        self.assertIn("analyze_failed", line)
        self.assertNotIn("isolation", line)
        self.assertEqual(row2["result"], "stopped_memory")  # a stop by the watchdog keeps its row, and is not an isolation failure
        self.assertNotIn("FAILED", row2["settings"])

    def test_sdkroot_from_the_environment(self):
        self.assertEqual(d.rust_sdkroot_args({}), [])
        self.assertEqual(d.rust_sdkroot_args({"BENCH_RUST_SDKROOT": ""}), [])
        self.assertEqual(d.rust_sdkroot_args({"BENCH_RUST_SDKROOT": "/x/MacOSX.sdk"}), ["-isolation-env", "SDKROOT=/x/MacOSX.sdk"])
        self.assertNotIn("SDKROOT=", open(d.TARGETS_FILE).read())  # never written in the data

    def test_sdkroot_reaches_the_command(self):
        w = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(w, ignore_errors=True))
        seen = []
        orig, orig_lock = d.run_watched, d.heavy_lock
        d.heavy_lock = lambda reason: __import__("contextlib").nullcontext()  # no real lock in a unit test
        d.run_watched = lambda cmd, *a, **k: (seen.append(cmd), {"rc": 1, "why": "", "wall": 0, "cpu": 0, "a": 0, "b": 0, "big": "", "big_rss": 0, "at_s": 0})[1]
        try:
            for sdk in ("/x/sdk", None):
                env = dict(os.environ)
                env.pop("BENCH_RUST_SDKROOT", None)
                if sdk:
                    env["BENCH_RUST_SDKROOT"] = sdk
                d.analyze_once("rivus", {"lang": "rust", "args": ["-timeout", "600s"]}, w, env, 10, 10, w, "t")
        finally:
            d.run_watched, d.heavy_lock = orig, orig_lock
        self.assertEqual(seen[0][-2:], ["-isolation-env", "SDKROOT=/x/sdk"])
        self.assertNotIn("-isolation-env", seen[1])

    def test_prepare_refuses_the_lock_for_a_crate_with_dependencies(self):
        r = self.tree({"Cargo.toml": self.TOML + '[dependencies]\nlibc = "0.2"\n'})
        w = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(w, ignore_errors=True))
        _, _, err = d.prepare("shlex", {"lang": "rust", "rust_minimal_lock": True}, r, w, dict(os.environ))
        self.assertIn("only for a crate with no dependency", err)


if __name__ == "__main__":
    unittest.main()
