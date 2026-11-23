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

    def test_worse(self):
        self.assertEqual(g.worse(0, 3), 3)
        self.assertEqual(g.worse(3, 1), 1)
        self.assertEqual(g.worse(1, 3), 1)
        self.assertEqual(g.worse(1, 2), 2)
        self.assertEqual(g.worse(2, 0), 2)
        self.assertEqual(g.worse(0, 0), 0)


if __name__ == "__main__":
    unittest.main()
