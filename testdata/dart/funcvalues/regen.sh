#!/bin/sh
# Makes testdata/ir/funcvalues.jsonl from this package: the Dart extractor, then `depMake` and its closure are marked
# as functions of a dependency (the fixture of the seeds: a closure made in a dependency is not a seed).
# Usage: testdata/dart/funcvalues/regen.sh  (from the root of the repository; needs dart)
set -e
dir=testdata/dart/funcvalues
(cd "$dir" && dart pub get --offline >/dev/null)
dart run extractor/dart/bin/rivus_dart.dart "$dir" | python3 -c '
import json, sys
for l in sys.stdin:
    d = json.loads(l)
    if d.get("kind") == "func" and "depMake" in d["func"]["id"]:
        d["func"]["dep"] = True
    print(json.dumps(d, separators=(",", ":")))
' > testdata/ir/funcvalues.jsonl
