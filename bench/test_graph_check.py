import json, os, sys, tempfile, unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import graph_check as g


class Items(unittest.TestCase):
    def write(self, d, name, findings):
        p = os.path.join(d, name)
        json.dump({"findings": findings, "uncertainties": []}, open(p, "w"))
        return p

    def test_same_and_different(self):
        with tempfile.TemporaryDirectory() as d:
            a = self.write(d, "a", [{"x": 1}, {"x": 2}])
            b = self.write(d, "b", [{"x": 2}, {"x": 1}])  # the order does not count
            c = self.write(d, "c", [{"x": 1}, {"x": 3}])
            self.assertEqual(g.compare_items("findings", a, b), ("findings: same (2)", True))
            line, same = g.compare_items("findings", a, c)
            self.assertFalse(same)
            self.assertIn("DIFFERENT (only in the first: 1, only in the second: 1)", line)
            self.assertFalse(g.compare_items("findings", a, os.path.join(d, "none"))[1])

    def test_order_only_differs(self):
        with tempfile.TemporaryDirectory() as d:
            a = self.write(d, "a", [{"x": 1, "y": [1, 2]}, {"x": 2}])
            b = self.write(d, "b", [{"x": 2}, {"x": 1, "y": [1, 2]}])  # only the order
            k = os.path.join(d, "k")
            json.dump({"findings": [{"y": [1, 2], "x": 1}, {"x": 2}], "uncertainties": []}, open(k, "w"))  # only the order of the keys
            self.assertTrue(g.compare_items("findings", a, b)[1])  # the set: same
            line, same = g.compare_order("findings", a, b)
            self.assertFalse(same)
            self.assertTrue(line.startswith("findings-order: DIFFERENT (2 in the first, 2 in the second"), line)
            self.assertTrue(g.compare_order("findings", a, k)[1])
            self.assertTrue(g.compare_order("uncertainties", a, b)[0].startswith("uncertainties-order: same (0"))
            self.assertFalse(g.compare_order("findings", a, os.path.join(d, "none"))[1])

    def test_worse(self):
        self.assertEqual(g.worse(0, 3), 3)
        self.assertEqual(g.worse(3, 1), 1)
        self.assertEqual(g.worse(1, 3), 1)
        self.assertEqual(g.worse(1, 2), 2)
        self.assertEqual(g.worse(2, 0), 2)
        self.assertEqual(g.worse(0, 0), 0)


class Convergence(unittest.TestCase):
    def test_line_and_status(self):
        self.assertEqual(g.convergence_line(False, False), (None, 0))
        line, rc = g.convergence_line(False, True)
        self.assertEqual(rc, 4)
        self.assertIn("NOT CONVERGED (analyze of head ended with exit code 4", line)
        self.assertIn("analyze of base and head", g.convergence_line(True, True)[0])
        self.assertEqual(g.worse(g.worse(0, 4), 1), 1)  # a difference stays a difference
        self.assertEqual(g.worse(0, 4), 4)
        self.assertEqual(g.worse(4, 3), 3)  # a part not compared is not hidden by the convergence
        self.assertEqual(g.worse(3, 4), 3)
        self.assertEqual(g.worse(4, 2), 2)


class CiGraphCheck(unittest.TestCase):
    """The weekly CI accepts exit 4 for zoo-go only (docs/issues/217): each target is its own call."""

    def test_only_zoo_go_may_end_with_4(self):
        root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        text = open(os.path.join(root, ".github", "workflows", "ci.yml")).read()
        calls = [l.strip() for l in text.splitlines() if l.strip().startswith("bench/compare.sh --graph-check")]
        self.assertEqual(len(calls), 3, calls)
        self.assertFalse(any("," in c.split("--graph-check ")[1].split()[0] for c in calls), "a target list in one call")
        for c in calls:
            target = c.split("--graph-check ")[1].split()[0]
            self.assertEqual("|| rc=$?" in c, target == "zoo-go", c)


class DartSteps(unittest.TestCase):
    """run_side with a Dart extractor: the IR is the output of the extractor, index and analyze get -lang dart and the same command in
    RIVUS_DART_EXTRACTOR, and an analyze that ends with exit code 4 is told (not an error)."""

    def test_dart_commands_and_exit_code_4(self):
        os.environ["RIVUS_HEAVY_LOCK_HELD"] = "1"  # no lock in a test
        with tempfile.TemporaryDirectory() as d:
            fake = os.path.join(d, "rivus")
            log = os.path.join(d, "log")
            open(fake, "w").write("#!/bin/sh\necho \"$@ EXT=$RIVUS_DART_EXTRACTOR\" >> %s\nif [ \"$1\" = analyze ]; then echo '{}'; : > \"$RIVUS_INTERNAL_SUMMARY_DUMP\"; exit 4; fi\n" % log)
            os.chmod(fake, 0o755)
            ext = os.path.join(d, "ext")
            open(ext, "w").write("#!/bin/sh\necho '{\"ir\":1}'\n")
            os.chmod(ext, 0o755)
            src = os.path.join(d, "src")
            os.mkdir(src)
            files, err, inc = g.run_side("base", fake, src, "", [], dict(os.environ), d, 60, 1 << 33, "t", loose=True, extractor=ext + " --flag")
            self.assertIsNone(err)
            self.assertTrue(inc)
            self.assertEqual(open(files["ir"]).read().strip(), '{"ir":1}')
            lines = open(log).read().splitlines()
            self.assertEqual(len(lines), 2, lines)  # index and analyze (the extractor is not the fake rivus)
            for l in lines:
                self.assertIn("-lang dart -C " + src, l)
                self.assertTrue(l.endswith("EXT=" + ext + " --flag"), l)
            files, err, inc = g.run_side("head", fake, src, "", [], dict(os.environ), d, 60, 1 << 33, "t", loose=True, extractor=os.path.join(d, "missing"))
            self.assertIn("extract could not start", err)


class RustTarget(unittest.TestCase):
    """-lang rust (zoo-rust): the extractor runs only under the sandbox of macOS, the isolation is checked each time, an IR is given to index and
    analyze (-ir), and the IR of the base extractor is made twice. No network and no real extractor: fake programs."""

    def setUp(self):
        os.environ["RIVUS_HEAVY_LOCK_HELD"] = "1"  # no lock in a test

    def fake(self, d, name, body):
        p = os.path.join(d, name)
        open(p, "w").write("#!/bin/sh\n" + body)
        os.chmod(p, 0o755)
        return p

    def test_not_run_without_the_sandbox(self):
        self.assertEqual(g.rust_unavailable("darwin", True), "")
        self.assertIn("no isolation", g.rust_unavailable("linux", True))
        self.assertIn("no isolation", g.rust_unavailable("darwin", False))
        with tempfile.TemporaryDirectory() as d:
            old, argv = g.rust_unavailable, sys.argv
            g.rust_unavailable = lambda *a: "this system has no isolation (test)"
            sys.argv = ["graph_check.py", "-name", "zoo-rust", "-lang", "rust", "-driver", "x", "-base-extractor", "x", "-head-extractor", "x",
                        "-base-bin", "x", "-head-bin", "x", "-graphcheck-bin", "x", "-dir", d, "-out", d]
            try:
                rc = g.main()
            finally:
                g.rust_unavailable, sys.argv = old, argv
            self.assertEqual(rc, 2)  # a run that did not run is 2, never 0 or 3
            self.assertIn("zoo-rust: NOT RUN (this system has no isolation", open(os.path.join(d, "graphcheck-zoo-rust.txt")).read())

    def test_failed_probe_stops_before_anything_runs(self):
        class A:
            base_extractor = head_extractor = driver = base_bin = head_bin = ""
            limit = 60
        old = g.db.sandbox_denies_write
        g.db.sandbox_denies_write = lambda d: (False, "a write under sandbox-exec was NOT denied (the file was made)")
        try:
            with tempfile.TemporaryDirectory() as d:
                why, pre, given, env = g.prepare_rust(A, d, "", [], {"K": "v"}, d, 1 << 33)
        finally:
            g.db.sandbox_denies_write = old
        self.assertTrue(why.startswith("isolation_failed: a write under sandbox-exec was NOT denied"), why)
        self.assertEqual(given, {})

    def test_rust_env_has_no_way_to_the_host(self):
        e = g.rust_env({"RIVUS_UNSAFE_HOST_EXEC": "1", "RIVUS_RUST_EXTRACTOR": "/other", g.DUMP_ENV: "/x", "PATH": "/bin", "HOME": "/real"}, "/empty", "/tc")
        self.assertEqual(e, {"PATH": "/bin", "HOME": "/empty", "RIVUS_RUST_TOOLCHAIN_DIR": "/tc"})

    def test_extract_runs_under_the_sandbox_with_the_given_extractor(self):
        with tempfile.TemporaryDirectory() as d:
            log = os.path.join(d, "log")
            drv = self.fake(d, "driver", "echo \"$@ HOME=$HOME HOST=${RIVUS_UNSAFE_HOST_EXEC-unset}\" >> %s\necho '{\"ir\":1}'\n" % log)
            env = g.rust_env({"PATH": "/usr/bin:/bin"}, os.path.join(d, "home"), "/tc")
            out, err = g.extract_rust("base", "1", drv, "/ext/rivus-rust", d, env, d, 60, 1 << 33)
            self.assertIsNone(err)
            self.assertEqual(open(out).read().strip(), '{"ir":1}')
            line = open(log).read()
            self.assertIn("debug extract -lang rust -C %s -rust-extractor /ext/rivus-rust" % d, line)
            self.assertNotIn("-unsafe-host-exec", line)
            self.assertIn("HOME=%s HOST=unset" % os.path.join(d, "home"), line)
            bad = self.fake(d, "bad", "exit 3\n")
            self.assertIn("extract failed (exit code 3", g.extract_rust("head", "1", bad, "/e", d, env, d, 60, 1 << 33)[1])
            empty = self.fake(d, "empty", "exit 0\n")
            self.assertIn("wrote nothing", g.extract_rust("head", "1", empty, "/e", d, env, d, 60, 1 << 33)[1])

    def test_isolation_is_read_from_the_report_of_each_side(self):
        with tempfile.TemporaryDirectory() as d:
            env = dict(os.environ)
            for mode, rc, want in (("sandbox", 0, None), ("sandbox", 4, None), ("host", 0, "isolation_failed: base side: summary.isolation.mode is 'host'"), (None, 0, "isolation_failed: base side")):
                summary = '{"summary":{"isolation":{"mode":"%s"}}}' % mode if mode else '{"summary":{}}'
                rivus = self.fake(d, "rivus", "echo '%s'\nexit %d\n" % (summary, rc))
                err = g.attest_isolation("base", rivus, "/ext", d, "", [], env, d, 60, 1 << 33)
                if want is None:
                    self.assertIsNone(err, (mode, rc))
                else:
                    self.assertTrue(err.startswith(want), (mode, err))
            failing = self.fake(d, "failing", "exit 1\n")
            self.assertIn("isolation check failed (exit code 1", g.attest_isolation("head", failing, "/ext", d, "", [], env, d, 60, 1 << 33))

    def test_index_and_analyze_read_the_given_ir(self):
        with tempfile.TemporaryDirectory() as d:
            log = os.path.join(d, "log")
            rivus = self.fake(d, "rivus", "echo \"$@\" >> %s\nif [ \"$1\" = analyze ]; then echo '{}'; : > \"$RIVUS_INTERNAL_SUMMARY_DUMP\"; exit 4; fi\n" % log)
            ir = os.path.join(d, "given.jsonl")
            open(ir, "w").write("{}\n")
            files, err, inc = g.run_side("base", rivus, d, "/r.yaml", [], dict(os.environ), d, 60, 1 << 33, "t", loose=True, workers="1", given_ir=ir)
            self.assertIsNone(err)
            self.assertTrue(inc)  # exit code 4 is told, not an error (state/global-chain stops by the pass limit on purpose)
            self.assertEqual(files["ir"], ir)
            lines = open(log).read().splitlines()
            self.assertEqual(len(lines), 2, lines)  # no extract step: the IR was made before
            self.assertTrue(lines[0].startswith("index -ir " + ir + " -C " + d), lines[0])
            self.assertTrue(lines[1].startswith("analyze -ir " + ir + " -C " + d), lines[1])
            for l in lines:
                self.assertNotIn("-lang", l)  # the model is chosen from the IR (checked on zoo-rust)
                self.assertIn("-workers 1", l)

    def test_ir_repro(self):
        with tempfile.TemporaryDirectory() as d:
            a, b, c = (os.path.join(d, n) for n in "abc")
            open(a, "w").write("x\n"); open(b, "w").write("x\n"); open(c, "w").write("y\n")
            line, rc = g.repro_line(a, b)
            self.assertEqual(rc, 0)
            self.assertTrue(line.startswith("ir-repro: the IR of the base extractor made twice is identical"), line)
            line, rc = g.repro_line(a, c)
            self.assertEqual(rc, 1)
            self.assertTrue(line.startswith("ir-repro: DIFFERENT"), line)

    def test_the_rows_of_a_rust_target(self):
        """The lines of the isolation and the reproducibility are at the top, a difference of the IR of the two extractors is a row (1), and exit 4 only adds the convergence row."""
        with tempfile.TemporaryDirectory() as d:
            gc = self.fake(d, "gc", "echo \"$1: same (3)\"\n")
            rep = os.path.join(d, "r.json")
            json.dump({"findings": [], "uncertainties": [], "stats": {}}, open(rep, "w"))
            dump = os.path.join(d, "dump")
            open(dump, "w").write("")
            side = {"ir": "a", "db": "b", "dump": dump, "report": rep}

            class A:
                graphcheck_bin = gc
            pre = [("isolation: sandbox (test)", 0), ("ir-repro: the IR of the base extractor made twice is identical (sha256 abc)", 0)]
            lines, status = g.compare(A, side, side, (True, True), pre)
            self.assertEqual(lines[:2], [t for t, _ in pre])
            self.assertEqual([l.split(":")[0] for l in lines[2:6]], ["ir", "edges", "summaries", "findings"])
            self.assertEqual(sum(1 for l in lines if ": same" in l and not l.startswith(("ir-repro", "isolation"))), 8)  # ir, edges, summaries, findings, uncertainties, 2 orders, scc_coarsened
            self.assertEqual(status, 4)  # same, and only the convergence is missing: the expected end of zoo-rust
            self.assertTrue(lines[-1].startswith("convergence: NOT CONVERGED (analyze of base and head"))
            lines, status = g.compare(A, side, side, (True, True), pre[:1] + [("ir-repro: DIFFERENT (x)", 1)])
            self.assertEqual(status, 1)  # a difference wins over the convergence


class ClockLine(unittest.TestCase):
    """A run that a time limit stopped (stats.load_dependent) is not a pass; the runs of the check get -func-timeout 24h."""

    def test_the_line_is_always_there_and_never_fails(self):
        with tempfile.TemporaryDirectory() as d:
            a, b = os.path.join(d, "a.json"), os.path.join(d, "b.json")
            json.dump({"stats": {"func_timeouts": 0}}, open(a, "w"))
            json.dump({"stats": {"load_dependent": True, "func_timeouts": 1}, "uncertainties": [{"detail": "func_timeout: x", "func": "go/types.Checker.builtin"}]}, open(b, "w"))
            line, rc = g.clock_line(a, a)
            self.assertEqual(rc, 0)
            self.assertTrue(line.startswith("func_timeouts: base 0"), line)
            line, rc = g.clock_line(a, b)
            self.assertEqual(rc, 0)
            self.assertTrue(line.startswith("::warning::"), line)
            self.assertIn("head 1 (", line)
            self.assertIn("go/types.Checker.builtin", line)

    def test_analyze_gets_the_func_timeout(self):
        os.environ["RIVUS_HEAVY_LOCK_HELD"] = "1"
        with tempfile.TemporaryDirectory() as d:
            fake = os.path.join(d, "rivus")
            log = os.path.join(d, "log")
            open(fake, "w").write("#!/bin/sh\necho \"$@\" >> %s\nif [ \"$1\" = analyze ]; then echo '{}'; : > \"$RIVUS_INTERNAL_SUMMARY_DUMP\"; fi\n" % log)
            os.chmod(fake, 0o755)
            src = os.path.join(d, "src")
            os.mkdir(src)
            files, err, inc = g.run_side("base", fake, src, "", ["./..."], dict(os.environ), d, 60, 1 << 33, "t")
            self.assertIsNone(err)
            an = [l for l in open(log).read().splitlines() if l.startswith("analyze")]
            self.assertEqual(len(an), 1)
            self.assertIn("-func-timeout " + g.FUNC_TIMEOUT, an[0])


class LockTimeout(unittest.TestCase):
    def test_busy_machine_is_not_run(self):
        import contextlib
        from heavylock import HeavyLockTimeout

        @contextlib.contextmanager
        def busy(reason):
            raise HeavyLockTimeout("gave up")
            yield

        old = g.db.heavy_lock
        g.db.heavy_lock = busy
        try:
            with tempfile.TemporaryDirectory() as d:
                files, err, inc = g.run_side("base", "/nonexistent", d, "", [], {}, d, 60, 1 << 33, "t", loose=True)
        finally:
            g.db.heavy_lock = old
        self.assertIn("did not run: gave up", err)


class WorkersSelfCheck(unittest.TestCase):
    """The same binary with 1 and 8 workers gives the same IR, edges, summaries and findings: the hash of the summaries does not hold the order of the work."""

    def test_one_and_eight_workers_are_the_same(self):
        import shutil, subprocess
        if not shutil.which("go"):
            self.skipTest("no go")
        root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        with tempfile.TemporaryDirectory() as d:
            for name, pkg in (("rivus", "./cmd/rivus"), ("graphcheck", "./bench/graphcheck")):
                subprocess.run(["go", "build", "-o", os.path.join(d, name), pkg], cwd=root, check=True)
            cur = os.path.join(root, "bench", "go-curated")
            p = subprocess.run([sys.executable, os.path.join(root, "bench", "graph_check.py"), "-name", "t", "-dir", cur, "-rules", os.path.join(cur, "rules.yaml"),
                                "-base-bin", os.path.join(d, "rivus"), "-head-bin", os.path.join(d, "rivus"), "-graphcheck-bin", os.path.join(d, "graphcheck"),
                                "-base-workers", "1", "-head-workers", "8", "-out", d], capture_output=True, text=True)
            self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
            self.assertEqual(p.stdout.count(": same"), 8, p.stdout)


class CoarsenLine(unittest.TestCase):
    """The line for stats.scc_coarsened (-scc-coarsen, ADR-124): 0 on both sides is same; a target that is hit differs."""

    def test_lines(self):
        import graph_check as g
        self.assertEqual(g.coarsen_line(0, 0)[1], 0)
        self.assertIn("same (0", g.coarsen_line(0, 0)[0])
        self.assertEqual(g.coarsen_line(1, 1)[1], 0)
        line, rc = g.coarsen_line(0, 1)
        self.assertEqual(rc, 1)
        self.assertIn("DIFFERENT", line)
        self.assertEqual(g.coarsen_line(None, 0)[1], 3)


if __name__ == "__main__":
    unittest.main()
