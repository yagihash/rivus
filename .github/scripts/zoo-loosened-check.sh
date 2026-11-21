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
# rows_added <old> <new>: how many rows the new file (a rev:path, or "-" for none) has that the old one has not, as a
# multiset. The columns that both have are compared (the old file has 4 columns and the new one 5: the first 4), so
# a column that is added is not a loosening; a column that goes away fails (prints -1).
cols() { git show "$1:$lock" | head -1 | awk -F'\t' '{print NF}'; }
rows_added() {
  local oc nc k
  if [ "$1" = "-" ]; then oc=$(cols "$2"); else oc=$(cols "$1"); fi
  nc=$(cols "$2")
  if [ "$nc" -lt "$oc" ]; then echo -1; return; fi
  k=$oc
  if [ "$1" = "-" ]; then
    git show "$2:$lock" | cut -f1-"$k" | wc -l | tr -d ' '
  else
    comm -13 <(git show "$1:$lock" | cut -f1-"$k" | LC_ALL=C sort) <(git show "$2:$lock" | cut -f1-"$k" | LC_ALL=C sort) | wc -l | tr -d ' '
  fi
}
for c in $(git rev-list "$base..$head"); do
  [ "$c" = "$first" ] && continue
  p=$(git rev-parse -q --verify "$c^1" || true)
  if [ -z "$p" ] || ! have "$p"; then
    if have "$c"; then added=$(rows_added - "$c"); else added=0; fi
  elif ! have "$c"; then
    echo "::error::$c deletes $lock"
    fail=1
    continue
  else
    added=$(rows_added "$p" "$c")
  fi
  if [ "$added" -lt 0 ]; then
    echo "::error::$c takes a column away from $lock"
    fail=1
    continue
  fi
  if [ "$added" -gt 0 ] && ! git log -1 --format=%B "$c" | grep -Eq "$approved"; then
    echo "::error::$c adds $added row(s) to $lock (a loosened mark) without a 'Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD' line in the message"
    fail=1
  fi
done
# the whole range: rows added in all, and no commit of the range has the line
if have "$base" && have "$head"; then
  net=$(rows_added "$base" "$head")
  if [ "$net" -gt 0 ] && ! git log --format=%B "$base..$head" | grep -Eq "$approved"; then
    echo "::error::$base..$head adds $net row(s) to $lock and no commit of the range has a Zoo-Loosened line"
    fail=1
  fi
fi
exit $fail
