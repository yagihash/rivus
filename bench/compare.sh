#!/usr/bin/env bash
# compare.sh <base> <head>: the accuracy of the core before and after a change, in one command.
# See usage() below, and bench/README.md ("Changing the accuracy of the core: the procedure").
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PY="$ROOT/bench/compare.py"

usage() {
  cat <<'EOF'
usage: bench/compare.sh [options] <base> <head>

Builds rivus at <base> and at <head> (git refs, e.g. c44bfdd and origin/main), runs both on the
benchmarks ONE AT A TIME (each benchmark: base, then head), and writes a table and the details of
every difference. Standard tools only (git, go, python3; dart / the Rust extractor for those targets).

Before each run it waits (every minute, at most --wait hours) until swap use is 50% or less and no
other rivus / rv-* / rivus-* process runs. Only one compare.sh runs at a time (a lock in the work dir).
Exit status: 0 nothing lost; 1 a finding was REMOVED (do not push: classify it); 2 a benchmark did not
run, did not build or did not finish (by the procedure, "not run" means "do not push").

options:
  --work DIR        clones, worktrees, binaries, results (default $COMPARE_WORK, else $TMPDIR/rivus-compare)
  --out DIR         where summary.md and details/ go (default <work>/results/<base7>-<head7>)
  --resume          keep a result that is already saved with exit code 0 (for a run that was stopped)
  --only a,b        run only these benchmarks      --skip a,b   do not run these
  --with-gitea      also run Gitea (v1.26.4: 10 to 16 minutes, 5 to 10 GB; off by default)
  --with-ech0       also run ech0 (may not finish; off by default)
  --wait HOURS      how long to wait for a quiet machine before a run (default 6)
  --timeout DUR     -timeout of rivus analyze, the same for base and head (default 1800s)
  --gomemlimit N=V  GOMEMLIMIT for benchmark N (repeatable; defaults: gogs-* 5GiB, gitea 10GiB)
  --dry-run         print what would be done and exit (nothing is created)
  --no-guard        do not wait for a quiet machine (for testing this script on the small benchmarks only)
  -h, --help

benchmarks (in this order): go-curated dart-curated rust-curated go-synth-S gogs-9a9388ac
gogs-48946299 gin-vue-admin go-test-bench parseable dart-pub [gitea] [ech0]
The inputs, the rules, the extractors (RIVUS_DART, RIVUS_RUST) are the ones of the working tree
this script is in, the same for base and head; only the rivus binary differs. A target whose copy
cannot be made (clone, checkout, go.mod, go list, pub get, no toolchain) is listed as NOT RUN.
nohup bench/compare.sh --resume c44bfdd origin/main > compare.log 2>&1 &
EOF
}

WORK="${COMPARE_WORK:-${TMPDIR:-/tmp}/rivus-compare}"
OUT=""; RESUME=0; ONLY=""; SKIP=""; WITH_GITEA=0; WITH_ECH0=0; WAIT=6; TIMEOUT=1800s; DRY=0; NOGUARD=0
MEMLIMITS="gogs-9a9388ac=5GiB gogs-48946299=5GiB gitea=10GiB"
POS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --work) WORK="$2"; shift ;;
    --out) OUT="$2"; shift ;;
    --resume) RESUME=1 ;;
    --only) ONLY="$2"; shift ;;
    --skip) SKIP="$2"; shift ;;
    --with-gitea) WITH_GITEA=1 ;;
    --with-ech0) WITH_ECH0=1 ;;
    --wait) WAIT="$2"; shift ;;
    --timeout) TIMEOUT="$2"; shift ;;
    --gomemlimit) MEMLIMITS="$MEMLIMITS $2"; shift ;;
    --dry-run) DRY=1 ;;
    --no-guard) NOGUARD=1 ;;
    -*) echo "unknown option $1" >&2; usage >&2; exit 64 ;;
    *) POS+=("$1") ;;
  esac
  shift
done
if [ "${#POS[@]}" -ne 2 ]; then usage >&2; exit 64; fi
BASE_REF="${POS[0]}"; HEAD_REF="${POS[1]}"

BASE_SHA="$(git -C "$ROOT" rev-parse --verify -q "$BASE_REF^{commit}")" || { echo "not a commit: $BASE_REF" >&2; exit 64; }
HEAD_SHA="$(git -C "$ROOT" rev-parse --verify -q "$HEAD_REF^{commit}")" || { echo "not a commit: $HEAD_REF" >&2; exit 64; }
B7="${BASE_SHA:0:7}"; H7="${HEAD_SHA:0:7}"
PAIR="$B7-$H7"
[ -n "$OUT" ] || OUT="$WORK/results/$PAIR"
RES="$OUT/raw"

BENCHES="go-curated dart-curated rust-curated go-synth-S gogs-9a9388ac gogs-48946299 gin-vue-admin go-test-bench parseable dart-pub"
[ "$WITH_GITEA" = 1 ] && BENCHES="$BENCHES gitea"
[ "$WITH_ECH0" = 1 ] && BENCHES="$BENCHES ech0"
if [ -n "$ONLY" ]; then BENCHES="$(echo "$ONLY" | tr ',' ' ')"; fi
if [ -n "$SKIP" ]; then
  keep=""
  for b in $BENCHES; do case ",$SKIP," in *",$b,"*) ;; *) keep="$keep $b" ;; esac; done
  BENCHES="$keep"
fi

# spec NAME: sets KIND (go|dart|rust|synth), URL, COMMIT, SUBDIR, RULES, DIR (the directory the analysis runs in), TRUTH
spec() {
  KIND=go; URL=""; COMMIT=""; SUBDIR=""; RULES=""; DIR=""; TRUTH=""; CLONE=""
  case "$1" in
    go-curated)    DIR="$ROOT/bench/go-curated"; RULES="$DIR/rules.yaml" ;;
    dart-curated)  KIND=dart; DIR="$ROOT/bench/dart-curated"; RULES="$DIR/rules.yaml" ;;
    rust-curated)  KIND=rust; DIR="$ROOT/bench/rust-curated"; RULES="$DIR/rules.yaml" ;;
    go-synth-S)    KIND=synth; DIR="$WORK/synth-S"; RULES="$DIR/rules.yaml" ;;
    gogs-9a9388ac) URL=https://github.com/gogs/gogs; COMMIT=8a3b8198a; CLONE=gogs; RULES="$ROOT/bench/gogs/rules.yaml"; TRUTH="$ROOT/bench/gogs/truth-9a9388ac.json" ;;
    gogs-48946299) URL=https://github.com/gogs/gogs; COMMIT=ca59e76a5; CLONE=gogs-2; RULES="$ROOT/bench/gogs/rules.yaml"; TRUTH="$ROOT/bench/gogs/truth-48946299.json" ;;
    gin-vue-admin) URL=https://github.com/flipped-aurora/gin-vue-admin; COMMIT=7bc7c3051067; CLONE=gin-vue-admin; SUBDIR=server
                   RULES="$ROOT/examples/rules/go-gin.yaml"; TRUTH="$ROOT/bench/gin-vue-admin/truth.json" ;;
    go-test-bench) URL=https://github.com/Contrast-Security-OSS/go-test-bench; COMMIT=""; CLONE=go-test-bench
                   RULES="$ROOT/examples/rules/go-gin.yaml"; TRUTH="$ROOT/bench/go-test-bench/truth.json" ;;
    parseable)     KIND=rust; URL=https://github.com/parseablehq/parseable; COMMIT=5c20b8e477be162238037e202cea1869d657017b; CLONE=parseable
                   RULES="$ROOT/bench/parseable/rules.yaml"; TRUTH="$ROOT/bench/parseable/truth-5c20b8e4.json" ;;
    dart-pub)      KIND=dart; URL=https://github.com/dart-lang/pub; COMMIT=5e24336e71a05a96b558d496953ebed223f6e40a; CLONE=pub
                   RULES="$ROOT/bench/dart-pub/rules.yaml"; TRUTH="$ROOT/bench/dart-pub/truth-5e24336e.json" ;;
    gitea)         URL=https://github.com/go-gitea/gitea; COMMIT=v1.26.4; CLONE=gitea
                   RULES="$ROOT/bench/gitea/rules.yaml"; TRUTH="$ROOT/bench/gitea/truth-v1.26.4.json" ;;
    ech0)          URL=https://github.com/lin-snow/Ech0; COMMIT=cecc2c19; CLONE=ech0 ;;  # repository and rules are not recorded: the built-in rules only
    *) return 1 ;;
  esac
  [ -n "$CLONE" ] && DIR="$WORK/src/$CLONE${SUBDIR:+/$SUBDIR}"
  return 0
}

memlimit_of() {
  local kv
  for kv in $MEMLIMITS; do [ "${kv%%=*}" = "$1" ] && ML="${kv#*=}"; done
}

if [ "$DRY" = 1 ]; then
  echo "compare: base $BASE_REF = $BASE_SHA"
  echo "         head $HEAD_REF = $HEAD_SHA"
  echo "work dir: $WORK    results: $OUT (raw: $RES)"
  echo "1. git worktree add --detach $WORK/wt/$B7 $B7 ; go build -o $WORK/bin/$PAIR/rivus-base ./cmd/rivus   (skipped if the binary exists)"
  echo "   git worktree add --detach $WORK/wt/$H7 $H7 ; go build -o $WORK/bin/$PAIR/rivus-head ./cmd/rivus"
  echo "2. benchmarks, one at a time, base then head; before each run: wait for swap <= 50% and no other rivus / rv-* / rivus-* process (at most $WAIT h)"
  for b in $BENCHES; do
    if ! spec "$b"; then echo "   $b: unknown benchmark" >&2; continue; fi
    ML=""; memlimit_of "$b"
    case "$KIND" in dart) ex="extractor: ${RIVUS_DART:-dart} run extractor/dart/bin/rivus_dart.dart";; rust) ex="extractor: ${RIVUS_RUST:-$ROOT/extractor/rust/target/debug/rivus-rust}";; *) ex="";; esac
    src="in-tree"; [ -n "$URL" ] && src="git clone $URL ; git checkout ${COMMIT:-<default branch>} ; ls go.mod ; go list ./..."
    [ "$KIND" = synth ] && src="go run ./bench/go-synth/gen -size S"
    printf '   %-14s %-5s %s | rules: %s | GOMEMLIMIT=%s | -timeout %s %s\n' "$b" "$KIND" "$src" "${RULES:-built-in}" "${ML:-unset}" "$TIMEOUT" "$ex"
  done
  [ "$WITH_GITEA" = 1 ] || echo "   (gitea is off: --with-gitea)"
  [ "$WITH_ECH0" = 1 ] || echo "   (ech0 is off: --with-ech0)"
  echo "3. python3 bench/compare.py report: $OUT/summary.md, $OUT/details/<bench>.md; exit 1 if a finding was removed, 2 if a benchmark did not run"
  exit 0
fi

mkdir -p "$WORK" "$RES" "$OUT/details" || exit 2

# one compare.sh at a time
LOCK="$WORK/lock.d"
if ! mkdir "$LOCK" 2>/dev/null; then
  if kill -0 "$(cat "$LOCK/pid" 2>/dev/null)" 2>/dev/null; then
    echo "another compare.sh is running (pid $(cat "$LOCK/pid")): the benchmarks are not run at the same time" >&2; exit 2
  fi
  /bin/rm -rf "$LOCK"; mkdir "$LOCK" || exit 2
fi
echo $$ > "$LOCK/pid"
trap '/bin/rm -rf "$LOCK"' EXIT

say() { echo "compare: $*" >&2; }
SKIPPED="$RES/skipped.tsv"; : > "$SKIPPED"; : > "$RES/scores.txt"
skip() { say "NOT RUN $1: $2"; printf '%s\t%s\n' "$1" "$2" >> "$SKIPPED"; }

# --- 1. the two binaries ---
BIN="$WORK/bin/$PAIR"; mkdir -p "$BIN"
build() { # label sha7 sha
  local label="$1" s7="$2" sha="$3" wt="$WORK/wt/$2"
  [ -x "$BIN/rivus-$label" ] && return 0
  if [ ! -d "$wt" ]; then
    mkdir -p "$WORK/wt"
    git -C "$ROOT" worktree add --detach "$wt" "$sha" >&2 || return 1
  fi
  say "building rivus-$label ($s7)"
  (cd "$wt" && go build -o "$BIN/rivus-$label" ./cmd/rivus) >&2 || return 1
}
if ! build base "$B7" "$BASE_SHA" || ! build head "$H7" "$HEAD_SHA"; then
  say "the build failed: nothing was measured"; exit 2
fi

# --- 2. the copies ---
clone_checkout() { # url commit dir [from-local]
  local url="$1" commit="$2" dir="$3"
  if [ ! -d "$dir/.git" ]; then
    mkdir -p "$(dirname "$dir")"
    git clone -q "$url" "$dir" >&2 || return 1
  fi
  [ -z "$commit" ] || git -C "$dir" checkout -q "$commit" >&2 || return 1
}

setup() { # name: makes the copy and the IR; sets IR (empty for Go); returns non-zero (and records why) if it cannot
  local b="$1" top; IR=""
  spec "$b" || { skip "$b" "unknown benchmark"; return 1; }
  if [ -n "$URL" ]; then
    top="$WORK/src/$CLONE"
    if [ "$CLONE" = gogs-2 ]; then
      [ -d "$WORK/src/gogs/.git" ] || clone_checkout "$URL" "" "$WORK/src/gogs" || { skip "$b" "git clone failed"; return 1; }
      [ -d "$top/.git" ] || git clone -q "$WORK/src/gogs" "$top" >&2 || { skip "$b" "git clone failed"; return 1; }
      git -C "$top" checkout -q "$COMMIT" >&2 || { skip "$b" "git checkout $COMMIT failed"; return 1; }
    else
      clone_checkout "$URL" "$COMMIT" "$top" || { skip "$b" "git clone / checkout $COMMIT failed"; return 1; }
    fi
    [ -n "$COMMIT" ] || say "$b: the commit is not recorded; HEAD of the default branch is $(git -C "$top" rev-parse --short HEAD)"
  fi
  case "$KIND" in
    go)
      if [ -n "$URL" ]; then
        [ -f "$DIR/go.mod" ] || { skip "$b" "no go.mod in $DIR"; return 1; }
        (cd "$DIR" && go list ./... > /dev/null 2>&1) || { skip "$b" "go list ./... failed in $DIR"; return 1; }
      fi ;;
    synth)
      [ -f "$DIR/rules.yaml" ] || (cd "$ROOT" && go run ./bench/go-synth/gen -size S -out "$DIR") >&2 || { skip "$b" "the generator failed"; return 1; } ;;
    dart)
      IR="$WORK/ir/$b.jsonl"; mkdir -p "$WORK/ir"
      local dartcmd="${RIVUS_DART:-dart}" nodeps="--no-deps"
      [ "$b" = dart-curated ] && nodeps=""
      (cd "$DIR" && $dartcmd pub get) > /dev/null 2>&1 || { skip "$b" "dart pub get failed (RIVUS_DART=$dartcmd)"; return 1; }
      [ -d "$ROOT/extractor/dart/.dart_tool" ] || (cd "$ROOT/extractor/dart" && $dartcmd pub get) > /dev/null 2>&1
      if [ ! -s "$IR" ]; then
        (cd "$ROOT/extractor/dart" && $dartcmd run bin/rivus_dart.dart $nodeps "$DIR" > "$IR") 2>"$WORK/ir/$b.err" || { /bin/rm -f "$IR"; skip "$b" "the Dart extractor failed (see $WORK/ir/$b.err)"; return 1; }
      fi ;;
    rust)
      IR="$WORK/ir/$b.jsonl"; mkdir -p "$WORK/ir"
      local rr="${RIVUS_RUST:-$ROOT/extractor/rust/target/debug/rivus-rust}"
      [ -x "$rr" ] || { skip "$b" "no Rust extractor at $rr (cargo build in extractor/rust, or RIVUS_RUST)"; return 1; }
      if [ ! -s "$IR" ]; then
        if [ "$b" = parseable ]; then
          mkdir -p "$WORK/parseable-assets"   # build.rs would fetch the UI from the network without it
          (cd "$DIR" && LOCAL_ASSETS_PATH="$WORK/parseable-assets" "$rr" --no-deps . > "$IR") 2>"$WORK/ir/$b.err" || { /bin/rm -f "$IR"; skip "$b" "the Rust extractor failed (see $WORK/ir/$b.err)"; return 1; }
        else
          (cd "$ROOT" && "$rr" "$DIR" > "$IR") 2>"$WORK/ir/$b.err" || { /bin/rm -f "$IR"; skip "$b" "the Rust extractor failed (see $WORK/ir/$b.err)"; return 1; }
        fi
      fi ;;
  esac
  return 0
}

# --- 3. the runs, one at a time: bench by bench, base then head ---
ALL=""
for b in $BENCHES; do
  ALL="$ALL,$b"
  setup "$b" || continue
  ML=""; memlimit_of "$b"
  rf=(); [ "$RESUME" = 1 ] && rf=(-resume)
  for w in base head; do
    say "$b: $w"
    args=(run -rivus "$BIN/rivus-$w" -dir "$DIR" -out "$RES/$b.$w" -timeout "$TIMEOUT" -wait "$WAIT" -gomemlimit "$ML")
    [ -n "$RULES" ] && args+=(-rules "$RULES")
    [ -n "$IR" ] && args+=(-ir "$IR")
    [ "$NOGUARD" = 1 ] && args+=(-no-guard)
    python3 "$PY" "${args[@]}" ${rf[@]+"${rf[@]}"} || true
  done
  if [ -n "$TRUTH" ]; then
    for w in base head; do
      [ -s "$RES/$b.$w.json" ] && python3 "$ROOT/bench/score.py" "$TRUTH" "$RES/$b.$w.json" 2>/dev/null | tail -n 2 | sed "s/^/$b $w: /" >> "$RES/scores.txt"
    done
  fi
done

# --- 4. the table ---
python3 "$PY" report -results "$RES" -base "$([ "$BASE_REF" = "$B7" ] && echo "$B7" || echo "$BASE_REF ($B7)")" -head "$([ "$HEAD_REF" = "$H7" ] && echo "$H7" || echo "$HEAD_REF ($H7)")" -out "$OUT" -benches "${ALL#,}" -skipped "$SKIPPED"
rc=$?
if [ -s "$RES/scores.txt" ]; then
  { echo; echo "Scores against the truth files (bench/score.py; the labelled sites):"; echo; sed 's/^/    /' "$RES/scores.txt"; } | tee -a "$OUT/summary.md"
fi
say "summary: $OUT/summary.md   exit $rc"
exit $rc
