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


def dry_run(docker_works, *benches):
    """compare.sh --dry-run with a stub `docker` first in PATH that answers (or not) to `docker info`."""
    with tempfile.TemporaryDirectory() as d:
        stub = os.path.join(d, "docker")
        with open(stub, "w") as f:
            f.write("#!/bin/sh\nexit %d\n" % (0 if docker_works else 1))
        os.chmod(stub, 0o755)
        env = dict(os.environ, PATH=d + os.pathsep + os.environ["PATH"], TMPDIR=d)
        env.pop("RIVUS_UNSAFE_HOST_EXEC", None)
        r = subprocess.run(["bash", os.path.join(HERE, "compare.sh"), "--dry-run", "--only", ",".join(benches), "HEAD", "HEAD"],
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


if __name__ == "__main__":
    unittest.main()
