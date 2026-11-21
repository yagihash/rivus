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


def dry_run(docker_works, *benches, extra=()):
    """compare.sh --dry-run with a stub `docker` first in PATH that answers (or not) to `docker info`."""
    with tempfile.TemporaryDirectory() as d:
        stub = os.path.join(d, "docker")
        with open(stub, "w") as f:
            f.write("#!/bin/sh\nexit %d\n" % (0 if docker_works else 1))
        os.chmod(stub, 0o755)
        env = dict(os.environ, PATH=d + os.pathsep + os.environ["PATH"], TMPDIR=d)
        env.pop("RIVUS_UNSAFE_HOST_EXEC", None)
        r = subprocess.run(["bash", os.path.join(HERE, "compare.sh"), "--dry-run", "--only", ",".join(benches)] + list(extra) + ["HEAD", "HEAD"],
                           capture_output=True, text=True, env=env)
        return r.returncode, r.stdout, r.stderr


def line(out, bench):
    return [l for l in out.splitlines() if l.startswith("   %s " % bench)][0]


class RustIsolation(unittest.TestCase):
    """Third-party Rust is read in the container only; our own code (bench/zoo-rust, bench/rust-curated) on the host."""

    def test_third_party_uses_the_container(self):
        rc, out, _ = dry_run(True, "parseable")
        self.assertEqual(rc, 0)
        self.assertIn("extractor: container", line(out, "parseable"))
        self.assertNotIn("unsafe-host-exec", line(out, "parseable"))

    def test_own_code_runs_on_the_host(self):
        rc, out, _ = dry_run(True, "rust-curated", "zoo-rust")
        self.assertEqual(rc, 0)
        for b in ("rust-curated", "zoo-rust"):
            self.assertIn("extractor: host (our own code: -unsafe-host-exec)", line(out, b))

    def test_no_docker_fails_the_third_party_only(self):
        rc, out, err = dry_run(False, "parseable")
        self.assertEqual(rc, 2)
        self.assertIn("Rust の第三者のコードは隔離の中でだけ読む。Docker を起動するか、このベンチを外す", err)
        rc, out, err = dry_run(False, "rust-curated", "zoo-rust")  # our own code needs no Docker
        self.assertEqual(rc, 0)

    def test_the_allowlist_is_by_path(self):
        script = 'eval "$(sed -n "/^OWN_RUST_PATHS=/,/^rust_mode()/p" %s/compare.sh)"; ' % HERE
        with tempfile.TemporaryDirectory() as root:
            for sub in ("bench/zoo-rust/cells", "bench/rust-curated", "extractor/rust/test/x", "bench/parseable", "src/zoo-rust-clone"):
                os.makedirs(os.path.join(root, sub))
            def mode(sub):
                return subprocess.run(["bash", "-c", script + 'rust_mode "$1" "$2"', "x", os.path.join(root, sub), root],
                                      capture_output=True, text=True).stdout.strip()
            self.assertEqual(mode("bench/zoo-rust/cells"), "host")
            self.assertEqual(mode("bench/rust-curated"), "host")
            self.assertEqual(mode("extractor/rust/test/x"), "host")
            self.assertEqual(mode("bench/parseable"), "container")
            self.assertEqual(mode("src/zoo-rust-clone"), "container")
            self.assertEqual(mode("does/not/exist"), "container")


class BigTargets(unittest.TestCase):
    URL = "https://example.invalid/o/r@" + "a" * 40

    def test_a_big_target_has_the_memory_limit_of_the_help(self):
        rc, out, _ = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL))
        self.assertEqual(rc, 0)
        self.assertIn("GOMEMLIMIT=10GiB", line(out, "bigone"))

    def test_gomemlimit_overrides_it(self):
        rc, out, _ = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL, "--gomemlimit", "bigone=3GiB"))
        self.assertIn("GOMEMLIMIT=3GiB", line(out, "bigone"))

    def test_the_light_ones_keep_theirs(self):
        rc, out, _ = dry_run(True, "gogs-9a9388ac", "casdoor")
        self.assertIn("GOMEMLIMIT=5GiB", line(out, "gogs-9a9388ac"))
        self.assertIn("GOMEMLIMIT=unset", line(out, "casdoor"))


class FixedInputsKey(unittest.TestCase):
    """--fixed-inputs reads the working tree: the key of an input is the hash of the working tree, not the one of HEAD."""

    def tree(self, root, fixed, path):
        script = 'ROOT="$1"; FIXED="$2"; eval "$(sed -n "/^wtree()/,/^h16()/p" %s/compare.sh)"; tree HEAD "$3"' % HERE
        r = subprocess.run(["bash", "-c", script, "x", root, fixed, path], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        return r.stdout.strip()

    def test_an_uncommitted_change_is_another_input(self):
        with tempfile.TemporaryDirectory() as root:
            def git(*a):
                subprocess.run(["git", "-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid"] + list(a), check=True, capture_output=True)
            git("init", "-q")
            os.makedirs(os.path.join(root, "bench/x"))
            f = os.path.join(root, "bench/x/a.txt")
            open(f, "w").write("one\n")
            git("add", ".")
            git("commit", "-q", "-m", "x")
            head_before, fixed_before = self.tree(root, "0", "bench/x"), self.tree(root, "1", "bench/x")
            self.assertTrue(head_before and fixed_before)
            open(f, "w").write("two\n")            # not committed
            self.assertEqual(self.tree(root, "0", "bench/x"), head_before)           # HEAD's tree: what was committed
            self.assertNotEqual(self.tree(root, "1", "bench/x"), fixed_before)       # the working tree: another input
            fixed_two = self.tree(root, "1", "bench/x")
            open(os.path.join(root, "bench/x/new.txt"), "w").write("n\n")           # an untracked file is an input too
            self.assertNotEqual(self.tree(root, "1", "bench/x"), fixed_two)
            self.assertEqual(self.tree(root, "1", "bench/none"), "")                  # no such input: empty, as the git form
            os.remove(f)                                                              # a tracked file that was removed
            self.assertNotEqual(self.tree(root, "1", "bench/x"), fixed_two)


if __name__ == "__main__":
    unittest.main()
