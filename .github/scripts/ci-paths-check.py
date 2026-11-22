#!/usr/bin/env python3
"""The check of .github/ci-paths.yml: every file of the repository (git ls-files) is in `code` or in `ignored`, exactly one of them.

A file is in a list when it matches one of its patterns and none of the patterns that start with '!' (what dorny/paths-filter does with
predicate-quantifier: some-with-excludes). Only the plain forms are allowed (a name, dir/**, dir/**/name, *.ext, dir/*/name), so that
this check and picomatch read a pattern the same way. It runs in the job `changes` of ci.yml, whatever changed.

usage: python3 .github/scripts/ci-paths-check.py [--self-test]
"""
import re
import subprocess
import sys

LISTS = ("code", "isolation", "ignored")
PLAIN = re.compile(r"^!?[A-Za-z0-9._/*-]+$")


def read_lists(path):
    lists, cur = {}, None
    for n, line in enumerate(open(path, encoding="utf-8"), 1):
        line = line.rstrip("\n")
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        m = re.match(r"^([a-z]+):\s*$", line)
        if m:
            cur = m.group(1)
            lists[cur] = []
            continue
        m = re.match(r"^  - '([^']*)'\s*$", line)
        if not m or cur is None:
            sys.exit("%s:%d: not the plain form (a key, or '  - 'pattern''): %r" % (path, n, line))
        if not PLAIN.match(m.group(1)):
            sys.exit("%s:%d: the pattern %r uses more than a name, dir/**, dir/**/name or *.ext" % (path, n, m.group(1)))
        lists[cur].append(m.group(1))
    for k in LISTS:
        if k not in lists:
            sys.exit("%s: the list %r is missing" % (path, k))
    return lists


def glob_re(p):
    out, i = "", 0
    while i < len(p):
        if p.startswith("**/", i):
            out += "(?:.*/)?"
            i += 3
        elif p.startswith("**", i):
            out += ".*"
            i += 2
        elif p[i] == "*":
            out += "[^/]*"
            i += 1
        else:
            out += re.escape(p[i])
            i += 1
    return re.compile("^" + out + "$")


def in_list(patterns, f):
    inc = [glob_re(p) for p in patterns if not p.startswith("!")]
    exc = [glob_re(p[1:]) for p in patterns if p.startswith("!")]
    return any(r.match(f) for r in inc) and not any(r.match(f) for r in exc)


def classify(lists, files):
    bad = []
    for f in files:
        c, i = in_list(lists["code"], f), in_list(lists["ignored"], f)
        if c == i:
            bad.append((f, "in both `code` and `ignored`" if c else "in neither `code` nor `ignored`"))
    return bad


def self_test():
    lists = {"code": ["cmd/**", "bench/**", "!bench/**/README.md"], "ignored": ["docs/**", "bench/**/README.md", "README.md"], "isolation": []}
    assert classify(lists, ["cmd/a.go", "bench/x/y.go", "bench/README.md", "bench/z/README.md", "docs/a.md", "README.md"]) == []
    assert classify(lists, ["renovate.json"]) == [("renovate.json", "in neither `code` nor `ignored`")]
    both = {"code": ["cmd/**"], "ignored": ["cmd/**"], "isolation": []}
    assert classify(both, ["cmd/a.go"]) == [("cmd/a.go", "in both `code` and `ignored`")]
    print("self-test ok")


def main():
    if "--self-test" in sys.argv:
        self_test()
        return
    lists = read_lists(".github/ci-paths.yml")
    files = subprocess.run(["git", "ls-files"], check=True, capture_output=True, text=True).stdout.splitlines()
    bad = classify(lists, files)
    for f, why in bad:
        print("%s: %s (.github/ci-paths.yml)" % (f, why), file=sys.stderr)
    if bad:
        sys.exit("%d file(s) are not in exactly one of the lists: put them in `code` (a change runs CI) or in `ignored` (it does not)" % len(bad))
    print("ok: %d files, every one in exactly one of `code` and `ignored`" % len(files))


main()
