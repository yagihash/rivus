#!/usr/bin/env python3
"""Scores the findings of `rivus analyze` against a truth file.

An entry is a sink site ("file", "line") or, when the sink is deep in the code
and the fix names the entry point, a "source_func" (a suffix of the function
the data comes from).

  rivus analyze ./... > result.json
  python3 bench/score.py bench/go-test-bench/truth.json result.json

A finding is matched by the position of its sink (file, line) and category.
Findings at sites the truth file does not label are listed, not counted.

A "source_func" entry may also say where the sink is ("sink_file" and "sink_line"): it is then matched by the pair
(source function, sink position), so that a finding from that function to some other sink does not count. An entry
with a "source_func" only does not look at the sink: every finding from that function with the category counts, so
the more a run over-reports, the better such an entry scores. The score says how many entries are of that kind
(the last lines; compare.sh copies them into scores.txt). Make the labels with the sink where it is known.
A label other than "vuln" or "safe", or an entry without a category, is an error (it would count as safe)."""
import json
import sys

truth = json.load(open(sys.argv[1]))["sites"]
res = json.load(open(sys.argv[2]))
for i, s in enumerate(truth):
    if s.get("label") not in ("vuln", "safe") or not s.get("category"):
        sys.exit("truth entry %d: label must be vuln or safe and category must be set: %r" % (i, s))
found = {}
by_source = {}
by_pair = {}
for f in res["findings"]:
    p = f["sink"]["position"]
    found.setdefault((p["file"], p["line"]), set()).add(f["category"])
    by_source.setdefault(f["source"]["func"], set()).add(f["category"])
    by_pair.setdefault((f["source"]["func"], p["file"], p["line"]), set()).add(f["category"])


def source_hit(suffix, category, sink=None):
    if sink:
        return any(fn.endswith(suffix) and (sf, sl) == sink and category in cats for (fn, sf, sl), cats in by_pair.items())
    return any(fn.endswith(suffix) and category in cats for fn, cats in by_source.items())

tp = fp = fn = tn = 0
src_only = 0
rows = []
labelled = set()
for s in truth:
    if "source_func" in s:
        sink = (s["sink_file"], s["sink_line"]) if "sink_file" in s and "sink_line" in s else None
        k = (s["source_func"], 0) if not sink else (s["source_func"] + " -> " + sink[0], sink[1])
        hit = source_hit(s["source_func"], s["category"], sink)
        if not sink:
            src_only += 1
    else:
        k = (s["file"], s["line"])
        hit = s["category"] in found.get(k, ())
    labelled.add(k)
    if s["label"] == "vuln":
        if hit:
            tp += 1; rows.append(("TP", k, s["category"]))
        else:
            fn += 1; rows.append(("FN", k, s["category"]))
    else:
        if hit:
            fp += 1; rows.append(("FP", k, s["category"]))
        else:
            tn += 1; rows.append(("TN", k, s["category"]))
for r in rows:
    print("%-3s %s:%d %s" % (r[0], r[1][0], r[1][1], r[2]))
if any("file" in t for t in truth):
    print()
    print("unlabelled sinks reported:")
    for k, cats in sorted(found.items()):
        if k not in labelled:
            print("  %s:%d %s" % (k[0], k[1], ",".join(sorted(cats))))
print()
rec = tp / (tp + fn) if tp + fn else 0
prec = tp / (tp + fp) if tp + fp else 0
print("TP %d  FN %d  FP %d  TN %d   recall %.2f  precision %.2f" % (tp, fn, fp, tn, rec, prec))
print("uncertainties: %d" % len(res["uncertainties"]))
if src_only:
    print("source-only entries: %d of %d (no sink in the truth: the sink is not checked, over-reporting does not lower the score)" % (src_only, len(truth)))
