#!/usr/bin/env python3
"""A trial of docs/DESIGN-observed-calls.md (not run by CI).

Reads the stacks that the Dart zoo recorded at its sinks (ZOO_STACKS=1, see README.md here) and the graph that
`rivus index` made of the cells (graph.db), and lists the calls that happened and that the graph does not have.

usage: compare.py stacks.json graph.db [--json out.json]
"""
import collections
import json
import re
import sqlite3
import sys

FRAME = re.compile(r'#\d+\s+(.*?) \(((?:package|dart|file):[^)]*?):(\d+):(\d+)\)')
CLOSURE = '<anonymous closure>'


def frames(stack):
    """[(name, uri, line, col)] from the sink outwards."""
    out = []
    for l in stack.split('\n'):
        m = FRAME.match(l.strip())
        if m:
            out.append((m.group(1), m.group(2), int(m.group(3)), int(m.group(4))))
    return out


SKIP = ('package:zoo_mark/', 'package:zoo_exec/', 'file:')  # the recording and the harness: not what rivus read
LINE_ONLY = '--line-only' in sys.argv
AT = collections.Counter()  # how the callee of an entry record was found (OBS-1)
KNOWN = set()  # the libraries that the graph has functions of (set in main)


def is_cell(uri):
    return uri in KNOWN and not uri.startswith(SKIP)


class Graph:
    def __init__(self, path):
        self.db = sqlite3.connect(path)
        self.funcs = {}  # id -> (fid, symbol, line, col)
        self.by_uri = collections.defaultdict(list)
        for i, fid, sym, line, col in self.db.execute('select id,fid,symbol,line,col from funcs'):
            self.funcs[i] = (fid, sym, line, col)
            self.by_uri[fid.split('#', 1)[0]].append((i, fid, sym, line, col))
        self.sites = collections.defaultdict(list)  # (uri, line) -> [(col, callsite id, kind, ncand)]
        for cid, line, col, kind, ncand, fid in self.db.execute(
                'select c.id,c.line,c.col,c.kind,c.ncand,f.fid from callsites c join funcs f on f.id=c.func_id'):
            self.sites[(fid.split('#', 1)[0], line)].append((col, cid, kind, ncand))
        self.edges = collections.defaultdict(set)  # callsite id -> {to_func}
        for cid, to in self.db.execute('select callsite_id,to_func from edges'):
            self.edges[cid].add(to)
        # A tear-off of a method (`m$bound`) is a closure that only calls the method. The VM records the method itself,
        # so a call site that reaches `m$bound` reaches the method one step on (only that one step, only from `$bound`).
        out = collections.defaultdict(set)
        for fr, to in self.db.execute('select from_func,to_func from edges'):
            out[fr].add(to)
        self.reach = {}  # callsite id -> {to_func} plus what the `$bound` ones call
        for cid, tos in self.edges.items():
            more = set()
            for t in tos:
                if self.funcs[t][0].endswith('$bound'):
                    more |= out[t]
            self.reach[cid] = tos | more
        self.refs = collections.defaultdict(int)  # target func -> number of references
        for (t,) in self.db.execute('select target_func from refs'):
            self.refs[t] += 1

    def callee(self, name, uri, line, col):
        """The function a frame is in: a list of candidate function ids (usually 1)."""
        cands = self.by_uri.get(uri, [])
        if name.endswith('.' + CLOSURE) or name == CLOSURE:
            parent = name[: -len(CLOSURE) - 1] if name != CLOSURE else ''
            parent = parent.replace('.' + CLOSURE, '')
            pre = uri + '#' + parent + '$'
            cs = [c for c in cands if c[1].startswith(pre) and (c[3], c[4]) <= (line, col)]
            if not cs:
                return []
            best = max((c[3], c[4]) for c in cs)
            return [c[0] for c in cs if (c[3], c[4]) == best]
        if name.startswith('new '):  # a constructor: "new Foo.named" is Foo.named, "new Foo" is Foo.new
            name = name[4:]
            if '.' not in name:
                name += '.new'
        want = uri + '#' + name
        got = [c[0] for c in cands if c[1] == want]
        parts = name.split('.')
        # a local function: "vuln1.f" or "nested.go.again" in the frame. The graph names it like a closure (vuln$1,
        # nested$1$1, by where it starts): of the ones with that many levels, the nearest one before the frame.
        for k in range(1, len(parts)):
            if got:
                break
            head = uri + '#' + '.'.join(parts[:k])
            depth = len(parts) - k
            cs = [c for c in cands if c[1].startswith(head + '$') and c[1][len(head):].count('$') == depth
                  and (c[3], c[4]) <= (line, col)]
            if cs:
                best = max((c[3], c[4]) for c in cs)
                got = [c[0] for c in cs if (c[3], c[4]) == best]
        return got

    def callee_at(self, uri, at):
        """OBS-1: the function that starts at `at` ("line:col" of the original file, which instrument.dart passed):
        exact, no guess from the name in the stack."""
        line, col = (int(x) for x in at.split(':'))
        return [c[0] for c in self.by_uri.get(uri, []) if (c[3], c[4]) == (line, col)]

    def callsite(self, uri, line, col):
        """The call sites of the line from the column of the frame on (the VM says where the name starts, the graph
        where the arguments start)."""
        if LINE_ONLY:  # method B moved the columns of the line (the instrumented file): the call sites of the line
            return list(self.sites.get((uri, line), []))
        cs = [s for s in self.sites.get((uri, line), []) if s[0] >= col]
        if not cs:
            return []
        best = min(s[0] for s in cs)
        return [s for s in cs if s[0] == best]


def main():
    stacks = json.load(open(sys.argv[1]))
    g = Graph(sys.argv[2])
    KNOWN.update(g.by_uri)
    seen = {}  # (kind, caller-site, callee) -> info
    counts = collections.Counter()
    for row in stacks:
        fs = frames(row['stack'])
        cells = [i for i, f in enumerate(fs) if is_cell(f[1])]
        for k, i in enumerate(cells):
            # the next frame outwards
            if i + 1 >= len(fs):
                continue
            nxt = fs[i + 1]
            callee = fs[i]
            at = row.get('at')
            if at and at != '0:0' and i == 0:
                ids = g.callee_at(callee[1], at)
                AT[('exact' if ids else 'at_unmatched')] += 1
            else:
                ids = g.callee(*callee)
            cname = '%s %s:%d' % (callee[0], callee[1], callee[2])
            if is_cell(nxt[1]):
                site = g.callsite(nxt[1], nxt[2], nxt[3])
                key = ('call', nxt[1], nxt[2], tuple(ids) or cname) if LINE_ONLY else ('call', nxt[1], nxt[2], nxt[3], tuple(ids) or cname)
                if key in seen:
                    seen[key]['cases'].add(row['case'])
                    continue
                if not ids:
                    status = 'callee_unresolved'
                elif not site:
                    status = 'no_callsite'
                elif any(set(ids) & g.reach.get(s[1], set()) for s in site):
                    status = 'ok'
                else:
                    status = 'no_edge'
                info = {'status': status, 'from': '%s %s:%d:%d' % (nxt[0], nxt[1], nxt[2], nxt[3]), 'to': cname,
                        'site_kind': sorted({s[2] for s in site}), 'site_ncand': [s[3] for s in site], 'cases': {row['case']}}
                seen[key] = info
            else:
                # called by the SDK or by the harness: a callback. The graph knows it only as a reference.
                # (the harness calls the cases, which are no edge of the cells: the top of the stack)
                up = next((f for f in fs[i + 1:] if is_cell(f[1])), None)
                if up is None:
                    continue
                key = ('sdk', up[1], up[2], up[3], tuple(ids) or cname)
                if key in seen:
                    seen[key]['cases'].add(row['case'])
                    continue
                if not ids:
                    status = 'callee_unresolved'
                else:
                    status = 'ok' if any(g.refs[i2] for i2 in ids) else 'no_ref'
                seen[key] = {'status': status, 'from': '%s %s:%d:%d (via %s)' % (up[0], up[1], up[2], up[3], nxt[0]),
                             'to': cname, 'site_kind': [], 'site_ncand': [], 'cases': {row['case']}}
    kinds = collections.Counter((k[0], v['status']) for k, v in seen.items())
    out = []
    for k, v in sorted(seen.items(), key=lambda kv: (kv[1]['status'], kv[0])):
        out.append(dict(v, kind=k[0], cases=sorted(v['cases'])))
    for (kind, st), n in sorted(kinds.items()):
        print('%-5s %-18s %d' % (kind, st, n))
    print('distinct observed edges: %d, stacks: %d' % (len(seen), len(stacks)))
    if AT:
        print('callee by the start position: %s' % dict(AT))
    if '--json' in sys.argv:
        json.dump(out, open(sys.argv[sys.argv.index('--json') + 1], 'w'), indent=1, ensure_ascii=False)
    if '--list' in sys.argv:
        want = sys.argv[sys.argv.index('--list') + 1]
        for o in out:
            if o['status'] == want:
                print(o['kind'], o['from'], '->', o['to'], o['site_kind'], o['site_ncand'], o['cases'][:2])


if __name__ == "__main__":
    main()
