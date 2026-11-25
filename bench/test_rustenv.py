#!/usr/bin/env python3
"""Tests of bench/rustenv.py (python3 bench/test_rustenv.py): combinations of environment variables. No rivus runs."""
import os, sys, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import rustenv as r


class DataDir(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.real = os.path.realpath(os.path.join(self.tmp.name, "realhome"))
        self.shared = os.path.join(self.real, "Library", "Application Support", "rivus")
        os.makedirs(self.shared)
        self.work = os.path.join(self.real, "tmp", "x")
        os.makedirs(self.work)
        self.addCleanup(self.tmp.cleanup)

    def check(self, env, platform="darwin"):
        r.require_rust_data_dir(env, "tool", real_home=self.real, platform=platform)

    def test_not_set_with_the_real_home_stops(self):
        with self.assertRaises(SystemExit) as c:
            self.check({"HOME": self.real})
        self.assertIn("RIVUS_RUST_DATA_DIR=~/tmp/", str(c.exception))

    def test_not_set_and_no_home_stops(self):
        with self.assertRaises(SystemExit):
            self.check({})

    def test_empty_value_is_not_set(self):
        with self.assertRaises(SystemExit):
            self.check({"HOME": self.real, "RIVUS_RUST_DATA_DIR": ""})

    def test_the_shared_place_stops(self):
        for d in (self.shared, os.path.join(self.shared, "rust"), os.path.join(self.shared, "rust", "x")):
            with self.assertRaises(SystemExit):
                self.check({"HOME": self.real, "RIVUS_RUST_DATA_DIR": d})

    def test_a_link_to_the_shared_place_stops(self):
        link = os.path.join(self.work, "link")
        os.symlink(self.shared, link)
        with self.assertRaises(SystemExit):
            self.check({"RIVUS_RUST_DATA_DIR": link})

    def test_a_place_under_tmp_passes(self):
        self.check({"HOME": self.real, "RIVUS_RUST_DATA_DIR": os.path.join(self.work, "data")})

    def test_a_sibling_with_the_same_prefix_passes(self):
        self.check({"RIVUS_RUST_DATA_DIR": self.shared + "-other"})

    def test_an_empty_home_without_the_variable_passes(self):
        # graph_check.rust_env and deps_bench.passline_run make HOME an empty directory: the default is under it. Not a regression.
        self.check({"HOME": self.work})

    def test_an_empty_home_with_the_shared_place_stops(self):
        with self.assertRaises(SystemExit):
            self.check({"HOME": self.work, "RIVUS_RUST_DATA_DIR": self.shared})

    def test_other_platform_uses_the_config_directory(self):
        with self.assertRaises(SystemExit):
            self.check({"HOME": self.real}, platform="linux")
        self.check({"HOME": self.real, "RIVUS_RUST_DATA_DIR": self.shared}, platform="linux")  # not the shared place there


if __name__ == "__main__":
    unittest.main()
