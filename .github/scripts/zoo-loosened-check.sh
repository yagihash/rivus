#!/usr/bin/env bash
# Usage: zoo-loosened-check.sh <base> <head>
# Fails if a commit of base..head adds a row to bench/zoo/marks.lock (a mark that loosens the truth of the zoo) and
# its message has no line "Zoo-Loosened: <reason>。ユーザーの承認: <date>". Rows that go away need nothing.
set -euo pipefail
base=$1
head=$2
lock=bench/zoo/marks.lock
# a new branch or a force push has no usable base: check the head commit only
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  range=$(git rev-parse "$head")
else
  range=$(git rev-list --no-merges "$base..$head")
fi
fail=0
for c in $range; do
  if git rev-parse -q --verify "$c^" >/dev/null; then
    added=$(git diff "$c^" "$c" -- "$lock" | grep -c '^+[^+]' || true)
  else
    added=$(git show "$c:$lock" 2>/dev/null | wc -l | tr -d ' ')
  fi
  if [ "$added" -gt 0 ] && ! git log -1 --format=%B "$c" | grep -Eq '^Zoo-Loosened: .+'; then
    echo "::error::$c adds $added row(s) to $lock (a loosened mark) without a 'Zoo-Loosened: <reason>。ユーザーの承認: <date>' line in the message"
    fail=1
  fi
done
exit $fail
