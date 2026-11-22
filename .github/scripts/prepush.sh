#!/usr/bin/env bash
# The check before a push (the same as CI: a language whose extractor is not there FAILS, it is not skipped).
#
#   .github/scripts/prepush.sh            go vet, the packages, and every test of cmd/rivus with the extractors
#   .github/scripts/prepush.sh '<regexp>' the tests of cmd/rivus that match, with the extractors (a change of a cell of the zoo, the core, the IR: use the
#                                         names of the three languages: 'Zoo|Curated|Dart|Rust|Builtin|Marks')
#
# It needs the Dart SDK (dart) and the Rust extractor (extractor/rust: cargo build; RIVUS_RUST names it if it is not at
# target/debug/rivus-rust). The Docker tests of the isolation are left to CI.
set -euo pipefail
cd "$(dirname "$0")/../.."
export RIVUS_DART="${RIVUS_DART:-dart}"
export RIVUS_RUST="${RIVUS_RUST:-$PWD/extractor/rust/target/debug/rivus-rust}"
export RIVUS_REQUIRE_DART=1 RIVUS_REQUIRE_RUST=1
command -v "${RIVUS_DART%% *}" >/dev/null || { echo "prepush: no dart ($RIVUS_DART)"; exit 1; }
[ -x "$RIVUS_RUST" ] || { echo "prepush: no Rust extractor at $RIVUS_RUST (cd extractor/rust && cargo build)"; exit 1; }
if [ $# -eq 0 ]; then
  go vet ./...
  go test -count=1 ./internal/... ./bench/...
  go test -count=1 -timeout 40m ./cmd/rivus
else
  go test -count=1 -timeout 40m -run "$1" ./cmd/rivus
fi
