"""Unit tests of compare.py (python3 -m unittest discover -s bench/zoo-dart/observed). Not run by CI."""
import importlib.util
import os
import sqlite3
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('compare', os.path.join(os.path.dirname(__file__), 'compare.py'))
compare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(compare)

U = 'package:t/a.dart'


def graph(funcs, callsites, edges):
    """funcs: [(id, name in the file, line, col)]; callsites: [(id, func id, line, col)];
    edges: [(callsite id, to func)] and [('f', from func, to func)] for an edge between functions."""
    f = tempfile.NamedTemporaryFile(suffix='.db', delete=False)
    f.close()
    db = sqlite3.connect(f.name)
    db.executescript('''create table funcs(id integer primary key, fid text, symbol text, line int, col int);
        create table callsites(id integer primary key, line int, col int, kind text, ncand int, func_id int);
        create table edges(callsite_id int, from_func int, to_func int);
        create table refs(target_func int);''')
    for i, fid, line, col in funcs:
        db.execute('insert into funcs values(?,?,?,?,?)', (i, U + '#' + fid, fid, line, col))
    for i, fn, line, col in callsites:
        db.execute('insert into callsites values(?,?,?,?,?,?)', (i, line, col, 'value', 1, fn))
    for e in edges:
        if e[0] == 'f':
            db.execute('insert into edges values(null,?,?)', (e[1], e[2]))
        else:
            db.execute('insert into edges values(?,null,?)', e)
    db.commit()
    db.close()
    g = compare.Graph(f.name)
    os.unlink(f.name)
    return g


class Bound(unittest.TestCase):
    def setUp(self):
        # main (1) has two call sites; the graph reaches Printer.run$bound (3) from the first and Other.run$bound (5)
        # from the second; each bound calls only its own method (2, 4).
        self.g = graph(
            [(1, 'main', 1, 1), (2, 'Printer.run', 10, 3), (3, 'Printer.run$bound', 12, 3),
             (4, 'Other.run', 20, 3), (5, 'Other.run$bound', 22, 3)],
            [(1, 1, 5, 3), (2, 1, 6, 3)],
            [(1, 3), (2, 5), ('f', 3, 2), ('f', 5, 4)])

    def test_bound_reaches_its_method(self):
        self.assertIn(2, self.g.reach[1])

    def test_bound_does_not_reach_another_method(self):
        self.assertNotIn(4, self.g.reach[1])
        self.assertNotIn(2, self.g.reach[2])

    def test_only_one_step(self):
        g = graph([(1, 'm', 1, 1), (2, 'A$bound', 2, 1), (3, 'B', 3, 1), (4, 'C', 4, 1)],
                  [(1, 1, 5, 3)], [(1, 2), ('f', 2, 3), ('f', 3, 4)])
        self.assertEqual(g.reach[1], {2, 3})

    def test_not_bound_is_not_followed(self):
        g = graph([(1, 'm', 1, 1), (2, 'A', 2, 1), (3, 'B', 3, 1)], [(1, 1, 5, 3)], [(1, 2), ('f', 2, 3)])
        self.assertEqual(g.reach[1], {2})


class Names(unittest.TestCase):
    def setUp(self):
        self.g = graph(
            [(1, 'nested', 51, 1), (2, 'nested$1', 53, 10), (3, 'nested$1$1', 54, 15), (4, 'vuln', 13, 1),
             (5, 'vuln$1', 15, 10), (6, 'Foo.new', 2, 3), (7, 'Foo.named', 4, 3), (8, 'Foo.go', 8, 3)],
            [], [])

    def test_nested_local_function(self):
        self.assertEqual(self.g.callee('nested.go.again', U, 55, 7), [3])
        self.assertEqual(self.g.callee('nested.go', U, 60, 5), [2])

    def test_local_function(self):
        self.assertEqual(self.g.callee('vuln.f', U, 16, 5), [5])

    def test_constructors(self):
        self.assertEqual(self.g.callee('new Foo', U, 2, 3), [6])
        self.assertEqual(self.g.callee('new Foo.named', U, 4, 3), [7])

    def test_plain_method(self):
        self.assertEqual(self.g.callee('Foo.go', U, 8, 3), [8])

    def test_unknown(self):
        self.assertEqual(self.g.callee('nested.zzz.q', U, 5, 1), [])


class Operators(unittest.TestCase):
    def setUp(self):
        # `b[0] = b[0]` on line 27: the graph has the `[]=` site at column 3 (the start of `b`) and the `[]` site at
        # column 10; the VM reports the `[]=` at column 4.
        self.g = graph(
            [(1, 'vuln', 25, 1), (2, 'Box.[]', 10, 3), (3, 'Box.[]=', 12, 3), (4, 'Box.run', 14, 3)],
            [(1, 1, 27, 3), (2, 1, 27, 10), (3, 1, 28, 3)],
            [(1, 3), (2, 2), (3, 4)])

    def test_operator_found_left_of_the_frame_column(self):
        self.assertEqual(self.g.callsite(U, 27, 4), [(10, 2, 'value', 1)])  # the plain lookup takes the other call
        self.assertEqual(self.g.operator_sites(U, 27, 4, [3], 'Box.[]='), [(3, 1, 'value', 1)])

    def test_the_other_operator_is_not_taken(self):
        self.assertEqual(self.g.operator_sites(U, 27, 4, [2], 'Box.[]'), [])  # `[]` is reached from column 10, right of the frame

    def test_a_plain_method_is_never_matched_to_the_left(self):
        self.assertEqual(self.g.operator_sites(U, 28, 9, [4], 'Box.run'), [])

    def test_setter_named_by_the_frame(self):
        # the graph names a setter `Box.x`, the VM `Box.x=`
        g = graph([(1, 'f', 1, 1), (2, 'Box.x', 5, 3)], [(1, 1, 9, 5)], [(1, 2)])
        self.assertEqual(g.operator_sites(U, 9, 7, [2], 'Box.x='), [(5, 1, 'value', 1)])
        self.assertEqual(g.operator_sites(U, 9, 7, [2], 'Box.x'), [])

    def test_is_operator_callee(self):
        for s in ('Box.[]', 'Box.[]=', 'Cls1.x=', 'A.==', 'A.+'):
            self.assertTrue(compare.is_operator_callee(s), s)
        for s in ('Box.run', 'Foo.new', 'vuln$1'):
            self.assertFalse(compare.is_operator_callee(s), s)


if __name__ == '__main__':
    unittest.main()
