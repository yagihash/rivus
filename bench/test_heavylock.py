#!/usr/bin/env python3
"""Tests of bench/heavylock.py (python3 bench/test_heavylock.py). Short limits; nothing heavy runs."""
import os, subprocess, sys, tempfile, time, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import heavylock as h

HOLD = "import sys,time,heavylock as h\nwith h.heavy_lock('t'):\n print('held',flush=True); time.sleep(float(sys.argv[1]))\n"


class Lock(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.env = dict(os.environ, RIVUS_HEAVY_LOCK=os.path.join(self.dir.name, "l"), RIVUS_HEAVY_LOCK_WAIT_S="3", PYTHONPATH=HERE)
        self.env.pop(h.HELD, None)
        self._old = {k: os.environ.get(k) for k in ("RIVUS_HEAVY_LOCK", "RIVUS_HEAVY_LOCK_WAIT_S", h.HELD)}
        os.environ["RIVUS_HEAVY_LOCK"] = self.env["RIVUS_HEAVY_LOCK"]
        os.environ.pop(h.HELD, None)

    def tearDown(self):
        for k, v in self._old.items():
            if v is None:
                os.environ.pop(k, None)
            else:
                os.environ[k] = v
        self.dir.cleanup()

    def holder(self, secs):
        p = subprocess.Popen([sys.executable, "-c", HOLD, str(secs)], env=self.env, stdout=subprocess.PIPE, text=True)
        self.assertEqual(p.stdout.readline().strip(), "held")
        return p

    def test_waits_and_says_so(self):
        p = self.holder(1)
        t0 = time.time()
        r = subprocess.run([sys.executable, os.path.join(HERE, "heavylock.py"), "--", sys.executable, "-c", "print('ran')"], env=self.env, capture_output=True, text=True)
        p.wait()
        self.assertEqual(r.returncode, 0)
        self.assertIn("ran", r.stdout)
        self.assertIn("別の重い実行を待っています", r.stderr)
        self.assertIn("PID %d" % p.pid, r.stderr)
        self.assertGreater(time.time() - t0, 0.5)

    def test_gives_up_at_the_limit(self):
        p = self.holder(10)
        try:
            self.env["RIVUS_HEAVY_LOCK_WAIT_S"] = "1"
            r = subprocess.run([sys.executable, os.path.join(HERE, "heavylock.py"), "--", sys.executable, "-c", "print('ran')"], env=self.env, capture_output=True, text=True)
            self.assertEqual(r.returncode, h.EXIT_TIMEOUT)
            self.assertNotIn("ran", r.stdout)
            self.assertIn("gave up", r.stderr)
        finally:
            p.kill(); p.wait()

    def test_dead_holder_releases(self):
        p = self.holder(10)
        p.kill(); p.wait()
        os.environ["RIVUS_HEAVY_LOCK_WAIT_S"] = "1"
        with h.heavy_lock("after kill", poll=0.05):
            pass

    def test_reentry_does_not_wait_for_itself(self):
        os.environ["RIVUS_HEAVY_LOCK_WAIT_S"] = "1"
        with h.heavy_lock("outer", poll=0.05):
            with h.heavy_lock("inner", poll=0.05):
                pass
            # a child started under the lock does not wait either
            r = subprocess.run([sys.executable, os.path.join(HERE, "heavylock.py"), "--", sys.executable, "-c", "print('ran')"], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stderr)
        self.assertNotIn(h.HELD, os.environ)

    def test_timeout_raises(self):
        p = self.holder(10)
        try:
            os.environ["RIVUS_HEAVY_LOCK_WAIT_S"] = "0.3"
            with self.assertRaises(h.HeavyLockTimeout):
                with h.heavy_lock("x", poll=0.05):
                    pass
        finally:
            p.kill(); p.wait()


if __name__ == "__main__":
    unittest.main()
