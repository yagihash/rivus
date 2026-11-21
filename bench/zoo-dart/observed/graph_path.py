#!/usr/bin/env python3
"""A trial of docs/DESIGN-observed-calls.md (OBS-2; not run by CI): for each observed edge that the graph does not have,
asks `rivus graph path` from the function that holds the call to the function that was called, and says what the graph
answers: reachable, or "no path" with exhaustive true (the answer that is wrong when the call really happened).

usage: graph_path.py zoo-B.json graph.db <rivus binary> [--no-refs]
"""
import collections, json, re, sqlite3, subprocess, sys

edges = json.load(open(sys.argv[1]))
db = sqlite3.connect(sys.argv[2])
rivus = sys.argv[3]
extra = [a for a in sys.argv[4:] if a != '--list']
LOC = re.compile(r'(?:(\S+) )?(package:[^ ]+?):(\d+)(?::(\d+))?')


def func_at(uri, line, col=None):
    """The function that starts last at or before the place, in the file."""
    r = db.execute('select fid from funcs where fid like ? and (line<? or (line=? and col<=?)) order by line desc, col desc limit 1',
                   (uri + '#%', line, line, col if col is not None else 10 ** 6)).fetchone()
    return r[0] if r else None


out = collections.Counter()
rows = []
for o in edges:
    if o['status'] not in ('no_edge', 'no_ref'):
        continue
    m1 = LOC.search(o['from'])
    m2 = LOC.search(o['to'])
    if not (m1 and m2):
        continue
    frm = func_at(m1.group(2), int(m1.group(3)), int(m1.group(4)) if m1.group(4) else None)
    to = func_at(m2.group(2), int(m2.group(3)))
    if not (frm and to):
        out['unknown'] += 1
        continue
    p = subprocess.run([rivus, 'graph', 'path', '-graph', sys.argv[2], '--from', frm, '--to', to] + extra, capture_output=True, text=True)
    try:
        d = json.loads(p.stdout)
    except Exception:
        out['error'] += 1
        continue
    if d.get('reachable'):
        k = 'reachable (%s)' % d.get('certainty')
    else:
        k = 'NO PATH, exhaustive=%s' % d.get('exhaustive')
    out[k] += 1
    rows.append((k, o['status'], frm.split('/')[-1], to.split('/')[-1]))
for k, n in sorted(out.items()):
    print('%-32s %d' % (k, n))
if '--list' in sys.argv:
    for r in sorted(rows):
        print(*r)
