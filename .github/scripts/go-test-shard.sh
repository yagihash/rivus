#!/bin/bash
# Runs one shard of the tests of one Go package: every test, once, in exactly one of the SHARD_TOTAL shards.
# The tests are the same as `go test PKG`; only the wall time is divided (the tests of a package run one after the other).
# usage: go-test-shard.sh PKG SHARD_INDEX SHARD_TOTAL [go test flags...]
set -euo pipefail
pkg=$1
index=$2
total=$3
shift 3
names=$(go test -list '.' "$pkg" | grep -E '^(Test|Example)' | LC_ALL=C sort | awk -v i="$index" -v n="$total" '(NR - 1) % n == i')
if [ -z "$names" ]; then
  echo "shard $index/$total of $pkg has no test" >&2
  exit 1
fi
echo "shard $index/$total of $pkg: $(echo "$names" | wc -l | tr -d ' ') tests"
regex="^($(echo "$names" | paste -sd'|' -))\$"
exec go test "$@" -run "$regex" "$pkg"
