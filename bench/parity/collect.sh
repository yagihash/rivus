#!/bin/bash
# Makes the inputs of `parity report`: the IR of each target (scanned into corpus-<name>.json) and, with
# `analyze`, the digest of `rivus analyze -pass-stats` (digest-<name>.json).
#
#   bench/parity/collect.sh [-n] STEP...        STEP: go-curated zoo-go gogs gva gophish casdoor
#                                                     dart-curated zoo-dart dart-pub rust-curated zoo-rust parseable
#
# Environment (all but WORK have a default or are only needed by the step that uses them):
#   WORK          where the files go (default $TMPDIR/rivus-parity)
#   SRC           the directory that has the checkouts: gogs gin-vue-admin gophish casdoor pub
#                 (see bench/README.md and bench/wild for the commits; fetch-sha.sh makes them)
#   RIVUS_DART    the command of the Dart SDK ("mise exec dart@3.13.6 -- dart")
#   RIVUS_RUST    the Rust extractor binary (extractor/rust: cargo build)
#   PARSEABLE_IR  an IR of parseable that already exists (the extraction is not run: it runs cargo on a
#                 project that is not ours, C-19)
#   ANALYZE=1     also run `rivus analyze -pass-stats` on the real targets (one at a time; minutes, GBs)
#
# One heavy step at a time: it waits when less than 40% of the memory is free.
set -eu
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
WORK=${WORK:-${TMPDIR:-/tmp}/rivus-parity}
SRC=${SRC:-$WORK/src}
mkdir -p "$WORK/ir" "$WORK/out"
RIVUS=${RIVUS:-$WORK/rivus}
PARITY=${PARITY:-$WORK/parity}
[ -x "$RIVUS" ] || (cd "$ROOT" && go build -o "$RIVUS" ./cmd/rivus)
(cd "$ROOT" && go build -o "$PARITY" ./bench/parity)

free_ok() {
  [ "$(uname)" = Darwin ] || return 0
  local f
  for _ in $(seq 1 120); do
    f=$(memory_pressure 2>/dev/null | sed -n 's/.*free percentage: \([0-9]*\)%.*/\1/p')
    [ -z "$f" ] || [ "$f" -ge 40 ] && return 0
    sleep 30
  done
  echo "memory stayed below 40% free" >&2
  return 1
}

scan() { # name lang real ir [note]
  "$PARITY" scan -name "$1" -lang "$2" ${3:+-real} -ir "$4" -out "$WORK/out/corpus-$1.json" -note "${5:-}"
}
digest() { # name lang json [note]
  "$PARITY" digest -name "$1" -lang "$2" -in "$3" -out "$WORK/out/digest-$1.json" -note "${4:-}"
}

go_target() { # name dir rules real [note]
  local name=$1 dir=$2 rules=$3 real=$4
  free_ok
  (cd "$dir" && "$RIVUS" extract ./... > "$WORK/ir/$name.jsonl")
  scan "$name" go "$real" "$WORK/ir/$name.jsonl" "${5:-}"
  if [ "${ANALYZE:-}" = 1 ] && [ -n "$rules" ]; then
    free_ok
    (cd "$dir" && "$RIVUS" analyze -pass-stats -rules "$rules" ./... > "$WORK/ir/$name.result.json" 2> "$WORK/ir/$name.err" || true)
    digest "$name" go "$WORK/ir/$name.result.json" "${5:-}"
  fi
}

dart_ir() { # name dir [extractor flags]  (the extractor reads the project at dir)
  local name=$1 dir=$2
  shift 2
  free_ok
  (cd "$ROOT/extractor/dart" && $RIVUS_DART run bin/rivus_dart.dart --no-deps "$@" "$dir" > "$WORK/ir/$name.jsonl")
}

analyze_ir() { # name lang rules
  [ "${ANALYZE:-}" = 1 ] || return 0
  free_ok
  "$RIVUS" analyze -pass-stats -ir "$WORK/ir/$1.jsonl" -rules "$3" > "$WORK/ir/$1.result.json" 2> "$WORK/ir/$1.err" || true
  digest "$1" "$2" "$WORK/ir/$1.result.json"
}

for step in "$@"; do
  case "$step" in
    go-curated) go_target go-curated "$ROOT/bench/go-curated" "" "" ;;
    zoo-go)     go_target zoo-go "$ROOT/bench/zoo-go" "" "" ;;
    gogs)       go_target gogs "$SRC/gogs" "$ROOT/bench/gogs/rules.yaml" 1 "8a3b8198a" ;;
    gva)        go_target gin-vue-admin "$SRC/gin-vue-admin/server" "$ROOT/examples/rules/go-gin.yaml" 1 "7bc7c3051067" ;;
    gophish)    go_target gophish "$SRC/gophish" "$ROOT/bench/wild/go/GHSA-9c9w-9pq7-f35h/rules.yaml" 1 "27d13a05" ;;
    casdoor)    go_target casdoor "$SRC/casdoor" "$ROOT/bench/wild/go/GHSA-9vm3-r8gq-cr6x/rules.yaml" 1 "7b0b426a" ;;
    dart-curated)
      dart_ir dart-curated "$ROOT/bench/dart-curated"
      scan dart-curated dart "" "$WORK/ir/dart-curated.jsonl" ;;
    zoo-dart)
      dart_ir zoo-dart "$ROOT/bench/zoo-dart/cells"
      scan zoo-dart dart "" "$WORK/ir/zoo-dart.jsonl" ;;
    dart-pub)
      dart_ir dart-pub "$SRC/pub"
      scan dart-pub dart 1 "$WORK/ir/dart-pub.jsonl" "5e24336e; the extractor leaves test/ out (its default, ADR-062)"
      analyze_ir dart-pub dart "$ROOT/bench/dart-pub/rules.yaml"
      # the same code with the tests, for the comparison with the old default
      dart_ir dart-pub-tests "$SRC/pub" --include-tests
      "$PARITY" scan -name dart-pub-tests -lang dart -real -supp -ir "$WORK/ir/dart-pub-tests.jsonl" -out "$WORK/out/corpus-dart-pub-tests.json" -note "5e24336e; --include-tests" ;;
    rust-curated)
      free_ok
      "$RIVUS_RUST" --no-deps "$ROOT/bench/rust-curated" > "$WORK/ir/rust-curated.jsonl"
      scan rust-curated rust "" "$WORK/ir/rust-curated.jsonl" ;;
    zoo-rust)
      # the workspace that rivus analyses (not the harness, not target/), as TestZooRust copies it
      rm -rf "$WORK/zoo-rust-ws" && mkdir -p "$WORK/zoo-rust-ws"
      for f in Cargo.toml Cargo.lock rules.yaml zoo-mark cells; do cp -R "$ROOT/bench/zoo-rust/$f" "$WORK/zoo-rust-ws/$f"; done
      rm -rf "$WORK/zoo-rust-ws/zoo-mark/target"
      free_ok
      "$RIVUS_RUST" "$WORK/zoo-rust-ws" > "$WORK/ir/zoo-rust.jsonl"
      scan zoo-rust rust "" "$WORK/ir/zoo-rust.jsonl" ;;
    parseable)
      cp "$PARSEABLE_IR" "$WORK/ir/parseable.jsonl"
      scan parseable rust 1 "$WORK/ir/parseable.jsonl" "5c20b8e4; an IR that was made before (not extracted again)"
      analyze_ir parseable rust "$ROOT/bench/parseable/rules.yaml" ;;
    *) echo "unknown step $step" >&2; exit 2 ;;
  esac
done
echo "inputs are in $WORK/out; then: $PARITY report -work $WORK/out -table bench/parity/TABLE.md -dead bench/parity/DEAD.md"
