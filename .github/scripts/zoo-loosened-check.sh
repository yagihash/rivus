#!/usr/bin/env bash
# Usage: zoo-loosened-check.sh <base> <head>
# Fails if a commit of base..head adds a row to bench/zoo/marks.lock (a mark that loosens the truth of the zoo) and
# its message has no line "Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD". Rows that go away need nothing.
# - the commit that made the file (ZOO_LOCK_FIRST) is the only one that is let off; any other commit whose parent has
#   no file counts every row as added, and a commit that deletes the file fails (delete and make again does not pass)
# - a merge commit is checked against its first parent, and the whole range (base..head) is checked once more
# - a base that is not usable (a new branch, a force push) fails: check it by hand
set -euo pipefail
base=${1:-}
head=${2:-}
lock=bench/zoo/marks.lock
first=${ZOO_LOCK_FIRST:-926ac45acac6dfd3e78f1a08027d7c56504e2999}
approved='^Zoo-Loosened: .+ユーザーの承認: [0-9]{4}-[0-9]{2}-[0-9]{2}'
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "::error::the base \`$base\` of this push is not usable: check the commits of $head by hand (git log -p -- $lock)"
  exit 1
fi
fail=0
have() { git cat-file -e "$1:$lock" 2>/dev/null; }
count_added() { grep -c '^+[^+]' || true; }
for c in $(git rev-list "$base..$head"); do
  [ "$c" = "$first" ] && continue
  p=$(git rev-parse -q --verify "$c^1" || true)
  if [ -z "$p" ] || ! have "$p"; then
    if have "$c"; then added=$(git show "$c:$lock" | wc -l | tr -d ' '); else added=0; fi
  elif ! have "$c"; then
    echo "::error::$c deletes $lock"
    fail=1
    continue
  else
    added=$(git diff "$p" "$c" -- "$lock" | count_added)
  fi
  if [ "$added" -gt 0 ] && ! git log -1 --format=%B "$c" | grep -Eq "$approved"; then
    echo "::error::$c adds $added row(s) to $lock (a loosened mark) without a 'Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD' line in the message"
    fail=1
  fi
done
# the whole range: rows added in all, and no commit of the range has the line
if have "$base" && have "$head"; then
  net=$(git diff "$base" "$head" -- "$lock" | count_added)
  if [ "$net" -gt 0 ] && ! git log --format=%B "$base..$head" | grep -Eq "$approved"; then
    echo "::error::$base..$head adds $net row(s) to $lock and no commit of the range has a Zoo-Loosened line"
    fail=1
  fi
fi
exit $fail
