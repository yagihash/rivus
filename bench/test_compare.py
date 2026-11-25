#!/usr/bin/env python3
"""Unit tests of bench/compare.py report (python3 bench/test_compare.py): the exit status of a surge of added findings,
a head that did not converge, and a clean pair. Synthetic results only; nothing is analysed."""
import json, os, shutil, subprocess, sys, tempfile, unittest

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


def dry_run(sandbox_works, *benches, extra=()):
    """compare.sh --dry-run with a stub `sandbox-exec` first in PATH (or, when the sandbox is to be missing, a PATH that has none: on macOS
    the real one is in /usr/bin, so the PATH is a directory of its own with the tools that the script needs and no sandbox-exec)."""
    with tempfile.TemporaryDirectory() as d:
        if sandbox_works:
            stub = os.path.join(d, "sandbox-exec")
            with open(stub, "w") as f:
                f.write("#!/bin/sh\nexit 0\n")
            os.chmod(stub, 0o755)
            path = d + os.pathsep + os.environ["PATH"]
        else:
            bindir = os.path.join(d, "bin")
            os.makedirs(bindir)
            for tool in ("bash", "sh", "git", "python3", "dirname", "basename", "cat", "env", "mkdir", "rm", "ls", "grep", "sed", "awk", "sort", "tr", "head", "tail", "wc", "cut", "uname", "date", "mktemp", "tee", "cp", "mv", "shasum", "sha256sum", "xargs", "printf", "readlink", "pwd", "id", "go", "cargo", "dart", "find", "diff", "expr", "true", "false", "test", "tar", "gzip"):
                src = shutil.which(tool)
                if src:
                    os.symlink(src, os.path.join(bindir, tool))
            path = bindir
        env = dict(os.environ, PATH=path, TMPDIR=d)
        env.pop("RIVUS_UNSAFE_HOST_EXEC", None)
        r = subprocess.run(["bash", os.path.join(HERE, "compare.sh"), "--dry-run", "--only", ",".join(benches)] + list(extra) + ["HEAD", "HEAD"],
                           capture_output=True, text=True, env=env)
        return r.returncode, r.stdout, r.stderr


def line(out, bench):
    return [l for l in out.splitlines() if l.startswith("   %s " % bench)][0]


class RustIsolation(unittest.TestCase):
    """Third-party Rust is read under the sandbox only; our own code (bench/zoo-rust, bench/rust-curated) on the host."""

    def test_third_party_uses_the_sandbox(self):
        rc, out, _ = dry_run(True, "parseable")
        self.assertEqual(rc, 0)
        self.assertIn("extractor: sandbox", line(out, "parseable"))
        self.assertNotIn("unsafe-host-exec", line(out, "parseable"))

    def test_own_code_runs_on_the_host(self):
        rc, out, _ = dry_run(True, "rust-curated", "zoo-rust")
        self.assertEqual(rc, 0)
        for b in ("rust-curated", "zoo-rust"):
            self.assertIn("extractor: host (our own code: -unsafe-host-exec)", line(out, b))

    def test_no_sandbox_fails_the_third_party_only(self):
        rc, out, err = dry_run(False, "parseable")
        self.assertEqual(rc, 2)
        self.assertIn("Rust の第三者のコードは隔離の中でだけ読む。macOS のサンドボックス(sandbox-exec)が使えないので、このベンチを外す", err)
        rc, out, err = dry_run(False, "rust-curated", "zoo-rust")  # our own code needs no sandbox
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
            self.assertEqual(mode("bench/parseable"), "sandbox")
            self.assertEqual(mode("src/zoo-rust-clone"), "sandbox")
            self.assertEqual(mode("does/not/exist"), "sandbox")


class BigTargets(unittest.TestCase):
    URL = "https://example.invalid/o/r@" + "a" * 40

    def test_a_big_target_has_the_memory_limit_of_the_help(self):
        rc, out, _ = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL))
        self.assertEqual(rc, 0)
        self.assertIn("GOMEMLIMIT=10GiB", line(out, "bigone"))

    def test_gomemlimit_overrides_it(self):
        rc, out, _ = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL, "--gomemlimit", "bigone=3GiB"))
        self.assertIn("GOMEMLIMIT=3GiB", line(out, "bigone"))

    def test_the_argument_is_checked(self):
        bad = {
            "a built-in name": "casdoor=" + self.URL,
            "white space (two words)": "big one=" + self.URL,
            "no =": "bigone",
            "no @": "bigone=https://example.invalid/o/r",
            "no URL": "bigone=nourl@" + "a" * 40,
            "an empty commit": "bigone=https://example.invalid/o/r@",
            "a commit with a space or a shell character": "bigone=https://example.invalid/o/r@a;b",
            "a name that is not a word": "-x=" + self.URL,
        }
        for why, arg in bad.items():
            rc, out, err = dry_run(True, "bigone", extra=("--big", arg))
            self.assertEqual(rc, 64, "%s: %r gave %d" % (why, arg, rc))
            self.assertIn("--big", err, why)
        rc, out, err = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL, "--big", "bigone=" + self.URL))
        self.assertEqual(rc, 64)
        self.assertIn("given twice", err)
        rc, out, err = dry_run(True, "bigone", extra=("--big", "bigone=" + self.URL, "--big", "two=" + self.URL))
        self.assertEqual(rc, 0, err)

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


class Cost(unittest.TestCase):
    """The cost table: the changes in percent, the 10% mark, the load average, and the cost-check file."""

    @classmethod
    def setUpClass(cls):
        sys.path.insert(0, HERE)
        import compare
        cls.c = compare

    def test_pct(self):
        self.assertEqual(self.c.pct(100, 112), "+12%")
        self.assertEqual(self.c.pct(100, 90), "-10%")
        self.assertEqual(self.c.pct(0, 5), "n/a")
        self.assertEqual(self.c.pct(None, 5), "n/a")

    def test_mark_needs_ten_percent_and_more_than_the_spread(self):
        m = self.c.marked
        self.assertTrue(m([100, 100], [120, 120], [1000, 1000], [1000, 1000]))      # CPU +20%, no spread
        self.assertTrue(m([100, 100], [100, 100], [1000, 1000], [1100, 1100]))      # peak +10%
        self.assertFalse(m([100, 100], [109, 109], [1000, 1000], [1099, 1099]))     # just below 10%
        self.assertFalse(m([100, 140], [130, 130], [1000, 1000], [1000, 1000]))     # +8% of medians, spread 33%
        self.assertFalse(m([100, 130], [135, 150], [1000], [1000]))                 # delta +24% but spread 26%: not a conclusion
        self.assertFalse(m([100, 100], [50, 50], [1000, 1000], [500, 500]))         # better
        self.assertFalse(m([100], [150], [1000], [1500]))                           # a single run: never marked
        self.assertFalse(m([], [110], [], [1100]))                                  # nothing to compare with

    def test_single_run_says_no_conclusion(self):
        row = self.c.cost_row("x", [{"cpu_s": 100, "max_rss_mb": 1000}], [{"cpu_s": 150, "max_rss_mb": 1500}], {}, {})
        self.assertIn("single run: no conclusion", row[2])
        self.assertEqual(row[6], "")

    def test_g12_warns_from_ten_percent(self):
        w = self.c.g12_warnings
        self.assertEqual(w("x", {"adds_total": 100, "passes": 1, "largest_scc": 35}, {"adds_total": 110, "passes": 2, "largest_scc": 56}),
                         ["G12: x: adds_total 100 -> 110 (+10%)", "G12: x: passes 1 -> 2 (+100%)", "G12: x: largest_scc 35 -> 56 (+60%)"])
        self.assertEqual(w("x", {"adds_total": 100, "adds_max_function_total": 100}, {"adds_total": 109, "adds_max_function_total": 50}), [])   # below 10%, and a fall
        self.assertEqual(w("x", {"adds_total": 0}, {"adds_total": 5}), [])   # base 0: no ratio
        self.assertEqual(w("x", {}, {"adds_total": 5}), [])   # a base without the number (an old version)

    def test_g12_columns_are_at_the_end(self):
        row = self.c.cost_row("x", [], [], {"adds_total": 10, "passes": 1}, {"adds_total": 12, "passes": 2})
        self.assertEqual(row[-4:], ["10 -> 12 (+20%)", "n/a", "1 -> 2 (+100%)", "n/a"])
        self.assertEqual(len(row), len(self.c.COST_HEADER[1].split("|")) - 2)

    def test_row_with_two_rounds(self):
        mb = [{"cpu_s": 100, "max_rss_mb": 1000, "wall_s": 50, "load_start": 1.5, "load_end": 2.0},
              {"cpu_s": 104, "max_rss_mb": 1010, "wall_s": 52, "load_start": 2.0, "load_end": 2.5}]
        mh = [{"cpu_s": 118, "max_rss_mb": 1000, "wall_s": 60, "load_start": 3.0, "load_end": 3.1},
              {"cpu_s": 122, "max_rss_mb": 1005, "wall_s": 61, "load_start": 3.1, "load_end": 3.4}]
        row = self.c.cost_row("x", mb, mh, {"summary_elems": 10, "summary_steps": 20}, {"summary_elems": 11, "summary_steps": 20})
        self.assertEqual(row[1], "2/2")
        self.assertEqual(row[2], "100.0-104.0 -> 118.0-122.0 (+18%, spread 4%)")
        self.assertEqual(row[4], "10 -> 11 (+10%)")
        self.assertEqual(row[6], "!!")
        self.assertEqual(row[8], "base 1.5/2.5, head 3.0/3.4")

    def test_row_of_an_old_cached_result_has_no_numbers(self):
        row = self.c.cost_row("x", [], [], {}, {})
        self.assertEqual(row[2], "n/a")
        self.assertEqual(row[6], "")

    def test_incomplete_run_is_marked_and_its_cost_is_not_compared(self):
        done = {"cpu_s": 100, "max_rss_mb": 1000, "wall_s": 50}
        stopped = {"cpu_s": 300, "max_rss_mb": 4000, "wall_s": 160, "exit_code": 4}
        row = self.c.cost_row("x", [done, done], [stopped, stopped], {}, {})
        self.assertEqual(row[6], "!! incomplete (head)")
        for i in (2, 3, 7):
            self.assertIn("incomplete run: no point comparing", row[i])
        self.assertNotIn("%", row[2])
        self.assertEqual(self.c.cost_row("x", [stopped], [done], {}, {})[6], "!! incomplete (base)")
        self.assertEqual(self.c.cost_row("x", [stopped], [stopped], {}, {})[6], "!! incomplete (base, head)")
        # a complete run on both sides (exit_code 0, or an old meta without it) is compared as before
        self.assertEqual(self.c.cost_row("x", [dict(done, exit_code=0)] * 2, [dict(done, cpu_s=130)] * 2, {}, {})[6], "!!")

    def test_report_marks_an_incomplete_run_in_the_summary_and_the_details(self):
        with tempfile.TemporaryDirectory() as d:
            write(d, "b", "base", 5)
            write(d, "b", "head", 5)
            def meta(name, **kw):
                with open(d + "/" + name, "w") as f:
                    json.dump(dict({"rc": 0, "wall_s": 1, "max_rss_mb": 100, "cpu_s": 10, "load_start": 1.0, "load_end": 1.0}, **kw), f)
            meta("b.base.meta.json"); meta("b.head.meta.json", exit_code=4, cpu_s=40)
            subprocess.run([sys.executable, os.path.join(HERE, "compare.py"), "report", "-results", d, "-base", "A", "-head", "B",
                            "-out", d + "/out", "-benches", "b"], capture_output=True, text=True)
            with open(d + "/out/summary.md") as f:
                text = f.read()
            with open(d + "/out/details/b.md") as f:
                details = f.read()
        self.assertIn("!! incomplete (head)", text.split("Cost, before -> after")[1])
        self.assertIn("incomplete run: no point comparing", text)
        self.assertIn("!! incomplete (head)", details)

    def test_report_has_the_cost_table_with_rounds_and_a_cost_check(self):
        with tempfile.TemporaryDirectory() as d:
            write(d, "b", "base", 5)
            write(d, "b", "head", 5, summary_elems=50)
            def meta(name, cpu):
                with open(d + "/" + name, "w") as f:
                    json.dump({"rc": 0, "wall_s": 1, "max_rss_mb": 100, "cpu_s": cpu, "load_start": 1.0, "load_end": 1.0}, f)
            meta("b.base.meta.json", 10); meta("b.head.meta.json", 12)
            meta("b.base.r2.meta.json", 10.2); meta("b.head.r2.meta.json", 12.1)
            run = {"cpu_s": 10, "max_rss_mb": 100, "wall_s": 1, "load_start": 1, "load_end": 1, "why": ""}
            side = {"runs": [run, run], "stats": {"summary_elems": 1, "summary_steps": 1}}
            with open(d + "/cost-memos.json", "w") as f:
                json.dump({"base": side, "head": side}, f)
            r = subprocess.run([sys.executable, os.path.join(HERE, "compare.py"), "report", "-results", d, "-base", "A", "-head", "B",
                                "-out", d + "/out", "-benches", "b"], capture_output=True, text=True)
            with open(d + "/out/summary.md") as f:
                text = f.read()
        self.assertEqual(r.returncode, 0)
        cost = text.split("Cost, before -> after")[1]
        self.assertIn("| b | 2/2 | 10.0-10.2 -> 12.0-12.1 (+19%, spread 2%)", cost)
        self.assertIn("!!", cost)
        self.assertIn("memos (cost check; peak = process-tree sum)", cost)


if __name__ == "__main__":
    unittest.main()
