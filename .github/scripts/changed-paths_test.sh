#!/usr/bin/env bash
# The test of changed-paths.sh: bash .github/scripts/changed-paths_test.sh
set -euo pipefail
cd "$(dirname "$0")"
fail=0
check() { # want-code want-isolation files...
  local wc="$1" wi="$2"; shift 2
  local got
  got="$(printf '%s\n' "$@" | GITHUB_OUTPUT= bash ./changed-paths.sh --stdin | tr '\n' ' ')"
  if [ "$got" != "code=$wc isolation=$wi " ]; then echo "FAIL: $* -> $got (want code=$wc isolation=$wi)"; fail=1; fi
}
check false false README.md
check false false docs/ROADMAP.md docs/issues/96-x.md
check false false examples/rules/README.md examples/a/b/NOTES.md
check false false extractor/rust/README.md extractor/dart/README.md bench/gcp/README.md bench/README.md bench/MISSES.md bench/gitea-cve-paths.md .claude/prompts/rivus-developer.md
check true true extractor/rust/NOTES.md.txt
check true false bench/BACKTEST.md
check true false bench/LEDGER.md
check true false CHANGELOG.md
check true false cmd/rivus/main.go
check true false cmd/rivus/main.go README.md
check false false docs/x.go README.md
check true true internal/sandbox/profile.go
check true true cmd/rivus/isolate_sandbox_darwin.go
check true true extractor/rust/src/main.rs
check true true go.mod
check true true .github/workflows/ci.yml
check false false extractor/dart/README.md docs/DESIGN-install.md
check true false extractor/dart/lib/extract.dart
check true true README.md internal/sandbox/x.go
got="$(echo cmd/rivus/main.go | EVENT_NAME=schedule GITHUB_OUTPUT= bash ./changed-paths.sh --stdin | tr '\n' ' ')"
[ "$got" = "code=false isolation=true " ] || { echo "FAIL: schedule -> $got"; fail=1; }
[ "$fail" = 0 ] && echo ok
exit "$fail"
