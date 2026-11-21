#!/usr/bin/env python3
"""How often does `rivus graph path` say "no path, exhaustive" (not a trial of the graph's quality: of what it claims)?
A fixed set of pairs: the functions of the file (own code, with a body), sorted by ID, and the k-th pair is chosen by
two strides (no randomness), so that two runs ask the same questions.

usage: exhaustive_rate.py graph.db <rivus binary> [pairs=500]
"""
import collections, sqlite3, subprocess, sys, json

db = sqlite3.connect(sys.argv[1])
rivus = sys.argv[2]
n_pairs = int(sys.argv[3]) if len(sys.argv) > 3 else 500
fids = [r[0] for r in db.execute('select fid from funcs where dep=0 and has_body=1 order by fid')]
n = len(fids)
out = collections.Counter()
seen = set()
k = 0
while len(seen) < min(n_pairs, n * (n - 1)) and k < n_pairs * 20:
    i, j = (k * 7919) % n, (k * 104729 + 13) % n
    k += 1
    if i == j or (i, j) in seen:
        continue
    seen.add((i, j))
    p = subprocess.run([rivus, 'graph', 'path', '-graph', sys.argv[1], '--from', fids[i], '--to', fids[j]], capture_output=True, text=True)
    try:
        d = json.loads(p.stdout)
    except Exception:
        out['error (ambiguous or unresolved target)'] += 1
        continue
    if d.get('reachable'):
        out['reachable'] += 1
    elif d.get('exhaustive'):
        out['no path, exhaustive=true'] += 1
    elif d.get('edges_exhausted'):
        out['no path, exhaustive=false (edges searched to the end)'] += 1
    else:
        out['no path, search cut'] += 1
print('functions %d, pairs %d' % (n, len(seen)))
for k, v in sorted(out.items()):
    print('  %-58s %d' % (k, v))
