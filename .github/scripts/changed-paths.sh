#!/usr/bin/env bash
# Which parts of CI a change needs. The first job of ci.yml runs this, and the heavy jobs read what it writes:
#   code=true       something that a test reads changed (anything but the Markdown of the list below): the heavy jobs run
#   isolation=true  what the sandbox of macOS is made of changed: the job of the runner of macOS runs
# It is what `paths-ignore` of ci.yml used to say, kept in one place: a change of only Markdown that no test reads does not run the heavy jobs.
# Not in the list on purpose: bench/BACKTEST.md, bench/LEDGER.md and bench/ledger-header.md (bench/test_backtest.py checks them), CHANGELOG.md.
#
#   changed-paths.sh                 BASE and HEAD_SHA (environment): the files that HEAD_SHA changed from BASE (a pull request: from the
#                                    merge base). Without a BASE that can be used (a schedule, a manual run, a first push), everything runs.
#   EVENT_NAME=schedule              only the runner of macOS runs
#   changed-paths.sh --stdin         the names of the files, one per line (the test: .github/scripts/changed-paths_test.sh)
set -euo pipefail

ignored='^(README\.md|docs/.*|examples/(.*/)?[^/]*\.md|extractor/[^/]+/README\.md|bench/(.*/)?README\.md|bench/MISSES\.md|bench/gitea-cve-paths\.md|\.claude/prompts/.*)$'
isolation='^(internal/sandbox/.*|cmd/rivus/isolate[^/]*\.go|extractor/rust/.*|go\.mod|go\.sum|\.github/workflows/ci\.yml|\.github/scripts/changed-paths\.sh)$'

emit() {
  echo "code=$1"
  echo "isolation=$2"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    { echo "code=$1"; echo "isolation=$2"; } >> "$GITHUB_OUTPUT"
  fi
}

# the weekly run is for the runner of macOS (the sandbox and its profile can break when macOS or Xcode is updated, with no change of the code)
if [ "${EVENT_NAME:-}" = "schedule" ]; then
  emit false true
  exit 0
fi

if [ "${1:-}" = "--stdin" ]; then
  files="$(cat)"
else
  base="${BASE:-}"
  head="${HEAD_SHA:-HEAD}"
  if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
    emit true true
    exit 0
  fi
  files="$(git diff --name-only "$base...$head" 2>/dev/null)" || { emit true true; exit 0; }
fi

if [ -z "$files" ]; then
  # nothing to compare: do not skip anything
  emit true true
  exit 0
fi

code=false
iso=false
while IFS= read -r f; do
  [ -z "$f" ] && continue
  # a file of the list is no change of the code, whatever else its path says (extractor/rust/README.md does not run the runner of macOS)
  if printf '%s\n' "$f" | grep -Eq "$ignored"; then continue; fi
  code=true
  if printf '%s\n' "$f" | grep -Eq "$isolation"; then iso=true; fi
done <<< "$files"
emit "$code" "$iso"
