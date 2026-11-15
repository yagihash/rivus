#!/usr/bin/env python3
"""Scores the findings of `rivus analyze` against a truth file.

  rivus analyze ./... > result.json
  python3 bench/score.py bench/go-test-bench/truth.json result.json

A finding is matched by the position of its sink (file, line) and category.
Findings at sites the truth file does not label are listed, not counted."""
import json
import sys

truth = json.load(open(sys.argv[1]))["sites"]
res = json.load(open(sys.argv[2]))
found = {}
for f in res["findings"]:
    p = f["sink"]["position"]
    found.setdefault((p["file"], p["line"]), set()).add(f["category"])

tp = fp = fn = tn = 0
rows = []
labelled = set()
for s in truth:
    k = (s["file"], s["line"])
    labelled.add(k)
    hit = s["category"] in found.get(k, ())
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
