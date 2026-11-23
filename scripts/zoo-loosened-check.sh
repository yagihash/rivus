#!/usr/bin/env bash
# Usage: zoo-loosened-check.sh <base> <head>
# What the commits base..head change in the marks of the zoo (bench/zoo/marks.d/<lang>/<cell file>.lock: one file for each
# cell file of the zoos), as a whole and against the merge base of the two (so the rows that a merge of main brings in are
# not "added": they are main's, and were checked there).
# A row is one mark of one case: <lang> TAB <file> TAB <kind> TAB <annotations> [TAB <the case>]. The file of a row is
# the cell; a lock file holds the rows of one cell, and its path is bench/zoo/marks.d/<lang>/<file>.lock (checked here).
# What needs "Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD" in the message of a commit of the range:
#   - a row added to a cell that the lock already has (a mark added to an existing cell, or a mark of it changed),
#   - a row of a new cell of a kind other than @sink(none), @miss and @fp (@noisy, @na, @todo, @manual).
#   - the marks that a Rust file writes outside its cells (`//! @na(...)` at the top of bench/zoo-rust/cells/src/lib.rs, lock
#     rust/lib.rs.lock) are rows too, so that they are in the same two cases.
# What does not, in any cell: a row of the kind todo-parity (`//! @todo(F/<cell>: PARITY: ...)` in the header of a Rust file:
# the cell that another language has and this one has not yet; the user decided it, 2026-11-23). A @todo that is not PARITY,
# and @na, @noisy, @manual there, need the line.
# What does not: the rows of a new cell (a lock file the merge base does not have yet) of @sink(none), @miss and @fp. A new
# @miss or @fp row must name an issue by @ref(#N) (the file docs/issues/N-*.md is there) or by @ref(model-<package>-<name>) (the
# file docs/issues/model-<package>-<name>.md is there, with the sections that docs/issues/README.md lists): that is checked
# here and nothing lets it off. Rows that go away need nothing.
# - a commit that deletes a lock file that is there at the head again fails (delete and make again does not pass), and so
#   does a column that goes away
# - a base that is not usable (a new branch, a force push) fails: check it by hand
# - the lock files of the merge base are read with the single file bench/zoo/marks.lock that the layout before the split had
set -euo pipefail
base=${1:-}
head=${2:-}
dir=bench/zoo/marks.d
legacy=bench/zoo/marks.lock
approved='^Zoo-Loosened: .+ユーザーの承認: [0-9]{4}-[0-9]{2}-[0-9]{2}'
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "::error::the base \`$base\` of this push is not usable: check the commits of $head by hand (git log -p -- $dir)"
  exit 1
fi
mb=$(git merge-base "$base" "$head" 2>/dev/null || true)
if [ -z "$mb" ]; then
  echo "::error::$base and $head have no merge base: check the commits of $head by hand (git log -p -- $dir)"
  exit 1
fi
fail=0
# the lock files of a commit
files() { git ls-tree -r --name-only "$1" -- "$dir" 2>/dev/null || true; }
# the rows of a commit: every lock file (and the single file of the old layout)
rows() {
  local f
  while IFS= read -r f; do
    [ -n "$f" ] && git show "$1:$f"
  done < <(files "$1")
  if git cat-file -e "$1:$legacy" 2>/dev/null; then git show "$1:$legacy"; fi
  return 0
}
mincols() { awk -F'\t' 'NF > 0 { if (m == 0 || NF < m) m = NF } END { print m + 0 }'; }

# the head: one file for a cell, at the path that its rows say
if git cat-file -e "$head:$legacy" 2>/dev/null; then
  echo "::error::$head has $legacy: the marks are in $dir (one file for a cell) since the split"
  fail=1
fi
headfiles=$(files "$head")
if [ -z "$headfiles" ]; then
  echo "::error::$head has no lock file in $dir"
  exit 1
fi
while IFS= read -r f; do
  if [[ ! "$f" =~ ^$dir/(go|dart|rust)/(.+)\.lock$ ]]; then
    echo "::error::$f is not <lang>/<cell file>.lock in $dir"
    fail=1
    continue
  fi
  want="${BASH_REMATCH[1]}"$'\t'"${BASH_REMATCH[2]}"
  bad=$(git show "$head:$f" | awk -F'\t' -v want="$want" 'NF > 0 && ($1 "\t" $2) != want { n++ } END { print n + 0 }')
  if [ "$bad" -gt 0 ]; then
    echo "::error::$f has $bad row(s) of another cell than the one in its path"
    fail=1
  fi
done <<<"$headfiles"
# a commit of the range that deletes a lock file that the head has again: a file that goes away and comes back would count every row as new
for c in $(git rev-list "$mb..$head"); do
  p=$(git rev-parse -q --verify "$c^1" || true)
  [ -z "$p" ] && continue
  for f in $(git diff-tree -r --no-commit-id --name-only --diff-filter=D "$p" "$c" -- "$dir"); do
    if git cat-file -e "$head:$f" 2>/dev/null; then
      echo "::error::$c deletes $f, and the head has it again"
      fail=1
    fi
  done
done

oldrows=$(rows "$mb")
newrows=$(rows "$head")
if [ -n "$oldrows" ]; then
  oc=$(printf '%s\n' "$oldrows" | mincols)
  nc=$(printf '%s\n' "$newrows" | mincols)
  if [ "$nc" -lt "$oc" ]; then
    echo "::error::$mb..$head takes a column away from the marks"
    exit 1
  fi
  k=$oc
  # the columns that both have are compared, so that a column that is added is not a loosening
  added=$(comm -13 <(printf '%s\n' "$oldrows" | cut -f1-"$k" | LC_ALL=C sort) <(printf '%s\n' "$newrows" | cut -f1-"$k" | LC_ALL=C sort) || true)
  cells=$(printf '%s\n' "$oldrows" | cut -f1,2 | LC_ALL=C sort -u)
else
  added=$(printf '%s\n' "$newrows" | LC_ALL=C sort)
  cells=""
fi

# the issues that the head has
issuefiles=$(git ls-tree --name-only "$head" -- docs/issues/ 2>/dev/null || true)
# issue_ok <ref>: the issue the @ref names has its file; one that is named has the sections of the template
issue_ok() {
  local r=$1 f
  case "$r" in
    '#'[0-9]*)
      [[ "${r#\#}" =~ ^[0-9]+$ ]] || return 1
      grep -Eq "^docs/issues/${r#\#}-[^/]+\.md$" <<<"$issuefiles"
      ;;
    model-*)
      [[ "$r" =~ ^model-[a-z0-9][a-z0-9._-]*$ ]] || return 1
      f="docs/issues/$r.md"
      grep -Fxq "$f" <<<"$issuefiles" || return 1
      local body
      body=$(git show "$head:$f")
      grep -Eq '^# .+' <<<"$body" && grep -Eq '^## 何が見逃されるか' <<<"$body" && grep -Eq '^## 原因' <<<"$body" && grep -Eq '^## 再現するセル' <<<"$body"
      ;;
    *) return 2 ;;
  esac
}
# ref_check <lang> <file> <kind> <annotations>: a new @miss or @fp row names an issue that is there
ref_check() {
  local lang=$1 file=$2 kind=$3 ann=$4 refs item found=0 bad=0 rc
  refs=$(grep -Eo '@ref\([^)]*\)' <<<"$ann" | sed -E 's/^@ref\((.*)\)$/\1/' | tr ',' '\n' || true)
  while IFS= read -r item; do
    [ -z "$item" ] && continue
    rc=0
    issue_ok "$item" || rc=$?
    case "$rc" in
      0) found=1 ;;
      1)
        echo "::error::a new @$kind row of $lang $file names $item: no such issue file in docs/issues (docs/issues/N-*.md, or model-<package>-<name>.md with the sections of docs/issues/README.md)"
        bad=1
        ;;
    esac
  done <<<"$refs"
  if [ "$found" = 0 ] && [ "$bad" = 0 ]; then
    echo "::error::a new @$kind row of $lang $file has no @ref(#N) or @ref(model-<package>-<name>) (name the issue)"
    bad=1
  fi
  [ "$bad" = 0 ]
}
loosened=0
while IFS=$'\t' read -r lang file kind ann _; do
  [ -z "${lang}${file}${kind}" ] && continue
  if [ "$kind" = "todo-parity" ]; then
    continue
  fi
  if grep -Fxq -- "$lang"$'\t'"$file" <<<"$cells"; then
    echo "row of an existing cell ($lang $file, $kind): a loosened mark"
    loosened=$((loosened + 1))
    continue
  fi
  case "$kind" in
    sink-none) ;;
    miss | fp)
      ref_check "$lang" "$file" "$kind" "$ann" || fail=1
      ;;
    *)
      echo "row of a new cell ($lang $file, $kind): a loosened mark"
      loosened=$((loosened + 1))
      ;;
  esac
done <<<"$added"
if [ "$loosened" -gt 0 ] && ! git log --format=%B "$mb..$head" | grep -Eq "$approved"; then
  echo "::error::$mb..$head adds $loosened loosened row(s) to the marks and no commit of the range has a 'Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD' line"
  fail=1
fi
exit $fail
