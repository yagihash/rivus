#!/usr/bin/env python3
"""A trial of docs/DESIGN-observed-calls.md (not run by CI).

For every @miss line of the sources: is there an observed edge that the graph lacks, in the same file?

usage: vs_miss.py edges.json <source root> <uri prefix, e.g. package:zoo_cells/> [-v]
"""
import glob
import json
import os
import re
import sys

edges = json.load(open(sys.argv[1]))
root, prefix = sys.argv[2], sys.argv[3]
bad = {}
for e in edges:
    if e['status'] == 'ok':
        continue
    for side in ('from', 'to'):
        m = re.search(re.escape(prefix) + r'(\S+?\.dart)', e[side])
        if m:
            bad.setdefault(m.group(1), []).append(e)
tot = hit = 0
for f in sorted(glob.glob(root + '/**/*.dart', recursive=True)):
    rel = os.path.relpath(f, root)
    for i, l in enumerate(open(f).read().split('\n'), 1):
        if '@miss' in l:
            tot += 1
            ok = rel in bad
            hit += ok
            if '-v' in sys.argv or not ok:
                print('HIT ' if ok else 'none', rel, i, re.search(r'@miss(\([^)]*\))?', l).group(0))
print('@miss lines: %d, in a file with an observed edge that the graph lacks: %d' % (tot, hit))
if '--others' in sys.argv:
    # the edges that the graph lacks in files with no @miss: new gaps, or a mistake of the method
    miss_files = {os.path.relpath(f, root) for f in glob.glob(root + '/**/*.dart', recursive=True) if '@miss' in open(f).read()}
    n = 0
    for e in edges:
        if e['status'] == 'ok':
            continue
        files = {m.group(1) for s in ('from', 'to') for m in [re.search(re.escape(prefix) + r'(\S+?\.dart)', e[s])] if m}
        if not files & miss_files:
            n += 1
            print('other', e['kind'], e['status'], e['from'].replace(prefix, ''), '->', e['to'].replace(prefix, ''), e['site_kind'], e['site_ncand'])
    print('edges that the graph lacks, in files with no @miss: %d' % n)
