#!/usr/bin/env bash
# Usage: shard.sh <index> <total>
# Prints the names of the tests (and examples) of cmd/rivus that shard <index> of <total> runs, one on each line.
# Every name goes to exactly one shard (the awk below gives each one a single value of s in 0..total-1), and a test that is added later
# is in some shard whatever its name: nothing is chosen by what a test needs (a test that needs Dart or Rust is in a shard like any other:
# every shard has the extractors when the change can affect them). `make shard-check` runs it for every index and compares the union with
# `go test -list`. The heavy ones (HEAVY, the heaviest first) are spread by their rank: the first one goes to shard 0, the others to the
# shards 1.. in turn (when there is more than one shard); the rest by their places in the sorted list, over all the shards.
# ALL="<names>" (sorted, one on each line) is used instead of `go test -list` (for the check, which lists once).
set -euo pipefail
i=${1:?index}
n=${2:?total}
HEAVY=${HEAVY:-"TestDeterministicDogfood TestDeterministicParamshare TestGCPRows TestDogfood TestDeterministicPassStats TestDeterministicCuratedGo TestScaleSynth"}
if [ -n "${ALL:-}" ]; then
  all=$ALL
else
  all=$(go test -list '.' ./cmd/rivus | grep -E '^(Test|Example)' | LC_ALL=C sort)
fi
echo "$all" | awk -v i="$i" -v n="$n" -v heavy="$HEAVY" '
  BEGIN { split(heavy, h, " "); for (k = 1; k in h; k++) rank[h[k]] = k - 1; spread = (n > 1) }
  $0 in rank {
    s = (!spread) ? rank[$0] % n : (rank[$0] == 0 ? 0 : 1 + (rank[$0] - 1) % (n - 1))
    if (s == i) print
    next
  }
  { s = r++ % n; if (s == i) print }'
