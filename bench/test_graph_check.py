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
        self.assertEqual(rc, 3)
        self.assertIn("NOT CONVERGED (analyze of head ended with exit code 4", line)
        self.assertIn("analyze of base and head", g.convergence_line(True, True)[0])
        self.assertEqual(g.worse(g.worse(0, 3), 1), 1)  # a difference stays a difference


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
            self.assertEqual(p.stdout.count(": same"), 7, p.stdout)


if __name__ == "__main__":
    unittest.main()
