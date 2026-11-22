#!/usr/bin/env bash
# Usage: zoo-loosened-check.sh <base> <head>
# What the commits base..head change in bench/zoo/marks.lock, as a whole and against the merge base of the two (so the
# rows that a merge of main brings in are not "added": they are main's, and were checked there).
# A row is one mark of one case: <lang> TAB <file> TAB <kind> TAB <annotations> [TAB <the case>]. The file of a row is
# the cell. What needs "Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD" in the message of a commit of the range:
#   - a row added to a cell that the lock already has (a mark added to an existing cell, or a mark of it changed),
#   - a row of a new cell of a kind other than @sink(none), @miss and @fp (@noisy, @na, @todo, @manual).
# What does not: the rows of a new cell (a file the lock does not have yet) of @sink(none), @miss and @fp. A new @miss or
# @fp row must name an issue (@ref(#N)): that is checked here and nothing lets it off. Rows that go away need nothing.
# - a commit that deletes the file fails (delete and make again does not pass), and so does a column that goes away
# - a base that is not usable (a new branch, a force push) fails: check it by hand
set -euo pipefail
base=${1:-}
head=${2:-}
lock=bench/zoo/marks.lock
approved='^Zoo-Loosened: .+ユーザーの承認: [0-9]{4}-[0-9]{2}-[0-9]{2}'
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "::error::the base \`$base\` of this push is not usable: check the commits of $head by hand (git log -p -- $lock)"
  exit 1
fi
mb=$(git merge-base "$base" "$head" 2>/dev/null || true)
if [ -z "$mb" ]; then
  echo "::error::$base and $head have no merge base: check the commits of $head by hand (git log -p -- $lock)"
  exit 1
fi
fail=0
have() { git cat-file -e "$1:$lock" 2>/dev/null; }
cols() { git show "$1:$lock" | head -1 | awk -F'\t' '{print NF}'; }
# a commit of the range that deletes the file: a file that goes away and comes back would count every row as new
for c in $(git rev-list "$mb..$head"); do
  p=$(git rev-parse -q --verify "$c^1" || true)
  if [ -n "$p" ] && have "$p" && ! have "$c"; then
    echo "::error::$c deletes $lock"
    fail=1
  fi
done
if ! have "$head"; then
  echo "::error::$head has no $lock"
  exit 1
fi
nc=$(cols "$head")
if have "$mb"; then
  oc=$(cols "$mb")
  if [ "$nc" -lt "$oc" ]; then
    echo "::error::$mb..$head takes a column away from $lock"
    exit 1
  fi
  k=$oc
  # the columns that both have are compared, so that a column that is added is not a loosening
  added=$(comm -13 <(git show "$mb:$lock" | cut -f1-"$k" | LC_ALL=C sort) <(git show "$head:$lock" | cut -f1-"$k" | LC_ALL=C sort) || true)
  cells=$(git show "$mb:$lock" | cut -f1,2 | LC_ALL=C sort -u)
else
  added=$(git show "$head:$lock" | LC_ALL=C sort)
  cells=""
fi
loosened=0
while IFS=$'\t' read -r lang file kind ann _; do
  [ -z "${lang}${file}${kind}" ] && continue
  if grep -Fxq -- "$lang"$'\t'"$file" <<<"$cells"; then
    echo "row of an existing cell ($lang $file, $kind): a loosened mark"
    loosened=$((loosened + 1))
    continue
  fi
  case "$kind" in
    sink-none) ;;
    miss | fp)
      if [[ "$ann" != *"@ref(#"* ]]; then
        echo "::error::a new @$kind row of $lang $file has no @ref(#N) (name the issue)"
        fail=1
      fi
      ;;
    *)
      echo "row of a new cell ($lang $file, $kind): a loosened mark"
      loosened=$((loosened + 1))
      ;;
  esac
done <<<"$added"
if [ "$loosened" -gt 0 ] && ! git log --format=%B "$mb..$head" | grep -Eq "$approved"; then
  echo "::error::$mb..$head adds $loosened loosened row(s) to $lock and no commit of the range has a 'Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD' line"
  fail=1
fi
exit $fail
