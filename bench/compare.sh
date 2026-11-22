#!/usr/bin/env bash
# compare.sh <base> <head>: the accuracy of the core before and after a change, in one command.
# See usage() below, and bench/README.md ("Changing the accuracy of the core: the procedure").
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PY="$ROOT/bench/compare.py"

usage() {
  cat <<'EOF'
usage: bench/compare.sh [options] <base> <head>

Builds rivus at <base> and at <head> (git refs, e.g. c9aa412 and origin/main), runs both on the
benchmarks ONE AT A TIME (each benchmark: base, then head), and writes a table and the details of
every difference. Standard tools only (git, go, python3; dart / the Rust extractor for those targets).

Before each run it waits (every minute, at most --wait hours) until the free memory is 40% or more (macOS; elsewhere swap use is 50% or less) and no
other rivus / rv-* / rivus-* process runs. Only one compare.sh runs at a time (a lock in the work dir).
Exit status: 0 nothing lost; 1 a finding was REMOVED (do not push: classify it); 2 a benchmark did not
run, did not build or did not finish, or a pair of runs is not a valid basis: head did not converge while base did, a run
timed out, stats.functions changed by 50% or more, or no finding in either (by the procedure, "not run" means "do not push").
Each such condition is a line at the top of summary.md; a change of 10% to 50% in stats.functions is a warning line.

options:
  --work DIR        clones, worktrees, binaries, results (default $COMPARE_WORK, else $TMPDIR/rivus-compare)
  --out DIR         where summary.md and details/ go (default <work>/results/<base7>-<head7>)
  --resume          keep a result that is already saved with exit code 0 (for a run that was stopped)
  --only a,b        run only these benchmarks      --skip a,b   do not run these
  --light           only the light tier: the benchmarks of one run of 60 seconds or less and 6 GB or less (the curated three, go-synth-S,
                    go-scale, zoo-go, zoo-dart, zoo-rust, gogs x 2, gin-vue-admin, go-test-bench, parseable, dart-pub, gophish, casdoor, and
                    gitea-cut: the cut-out of Gitea's three CVEs, 7 packages, not the whole of Gitea). Base and head of a benchmark run at the same
                    time when the free memory allows it; a light run does not wait for other analyses, only for free memory (25%)
  --big NAME=URL@COMMIT  a big real codebase, run once per side (repeatable; NAME is a short word, COMMIT a full SHA or a tag; Go, the
                    built-in rules). One at a time, waits for a quiet machine, never uses the cache. For the periodic and the
                    occasional runs (bench/explore/README.md), never the gate. With --big and neither --light nor --heavy, the light
                    tier runs first; --heavy runs only the --big targets
  --heavy           only the --big targets (see above)
  --no-cache        do not keep or use the results of base (see below)
  --cache-dir DIR   where they are kept (default <work>/cache)
  --wait HOURS      how long to wait for a quiet machine before a run (default 6)
  --timeout DUR     -timeout of rivus analyze, the same for base and head (default 1800s)
  --gomemlimit N=V  GOMEMLIMIT for benchmark N (repeatable; defaults: gogs-* 5GiB, a --big target 10GiB)
  --fixed-inputs    the backtest (bench/BACKTEST.md): both sides are analysed with the inputs of THIS tree (fixtures, rules, generator,
                    extractor), only the engine is the one of <base> / <head>, so that an old engine can be judged on today's inputs
  --dry-run         print what would be done and exit (nothing is created)
  --no-guard        do not wait for a quiet machine (for testing this script on the small benchmarks only)
  -h, --help

The result of BASE in the light tier is kept (<work>/cache/<key>) and used by the next comparison with the same base: the analysis is
deterministic, so the same binary (its sha256), target state (commit, clean tree, go.sum, go version and GOOS/GOARCH/GOFLAGS for
Go; the sha256 of the IR for Dart and Rust), rules (their content) and arguments give the same result. A run that a clock decided
(timed_out, func_timeouts, budget_exhausted) or that did not converge is not kept; a dirty tree is not cached; the material of the
key is kept with the result and compared when it is read. summary.md says "base: cached <date>".

benchmarks (in this order): go-curated dart-curated rust-curated go-synth-S gogs-9a9388ac
gogs-48946299 gin-vue-admin go-test-bench parseable dart-pub gophish casdoor gitea-cut
Each side is analysed with the inputs of ITS OWN version: the fixtures, the rules, the synth generator and
the extractor are taken from wt/<sha7> (the worktree of that commit). The IR is cached per version, with a key made of
the benchmark, the tree hash of the extractor (git rev-parse <sha>:extractor/<lang>), the tree hash of the target (or
the commit of the clone) and the flags; the same input is not extracted twice. The Rust extractor of each version is built
once (cargo build --locked, shared target dir). RIVUS_RUST (a binary) or RIVUS_DART (a command) name the tool for both
sides. summary.md lists, per benchmark, the input and the IR sha256 of both sides. A target whose copy
cannot be made (clone, checkout, go.mod, go list, pub get, no toolchain) is listed as NOT RUN.
nohup bench/compare.sh --resume c9aa412 origin/main > compare.log 2>&1 &
EOF
}

WORK="${COMPARE_WORK:-${TMPDIR:-/tmp}/rivus-compare}"
OUT=""; RESUME=0; ONLY=""; SKIP=""; BIGS=""; SEL_LIGHT=0; SEL_HEAVY=0; NOCACHE=0; FIXED=0; CACHE_DIR=""; WAIT=6; TIMEOUT=1800s; DRY=0; NOGUARD=0
MEMLIMITS="gogs-9a9388ac=5GiB gogs-48946299=5GiB gitea-cut=4GiB"
POS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --work) WORK="$2"; shift ;;
    --out) OUT="$2"; shift ;;
    --resume) RESUME=1 ;;
    --only) ONLY="$2"; shift ;;
    --skip) SKIP="$2"; shift ;;
    --light) SEL_LIGHT=1 ;;
    --heavy) SEL_HEAVY=1 ;;
    --no-cache) NOCACHE=1 ;;
    --cache-dir) CACHE_DIR="$2"; shift ;;
    --big) [ $# -ge 2 ] || { echo "--big needs NAME=URL@COMMIT" >&2; exit 64; }
           case "$2" in *[[:space:]]*) echo "--big: no white space in '$2' (NAME=URL@COMMIT is one word)" >&2; exit 64 ;; esac
           BIGS="$BIGS $2"; shift ;;
    --wait) WAIT="$2"; shift ;;
    --timeout) TIMEOUT="$2"; shift ;;
    --gomemlimit) MEMLIMITS="$MEMLIMITS $2"; shift ;;
    --fixed-inputs) FIXED=1 ;;
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

# The tiers are measured, not named: the light one is a run of 60 seconds or less and 6 GB or less (gogs 11 to 27 s and 3.3 to 3.8 GB,
# gin-vue-admin 21 s and 5.3 GB, go-test-bench 4 s, parseable 3 s, dart-pub 2 s); the heavy one is a --big target.
is_heavy() { local kv; for kv in $BIGS; do [ "${kv%%=*}" = "$1" ] && return 0; done; return 1; }
big_names() { local kv; for kv in $BIGS; do printf '%s ' "${kv%%=*}"; done; }
BENCHES="go-curated dart-curated rust-curated go-synth-S"
grep -q "^    go-scale)" "${BASH_SOURCE[0]}" && BENCHES="$BENCHES go-scale"   # a pseudo-project of scale, added by another change
grep -q "^    zoo-go)" "${BASH_SOURCE[0]}" && BENCHES="$BENCHES zoo-go"   # the zoo of Go: cases with answers that a run backs (bench/zoo/README.md)
grep -q "^    zoo-dart)" "${BASH_SOURCE[0]}" && BENCHES="$BENCHES zoo-dart"   # the zoo of Dart: the same
grep -q "^    zoo-rust)" "${BASH_SOURCE[0]}" && BENCHES="$BENCHES zoo-rust"   # the zoo of Rust: the same
BENCHES="$BENCHES gogs-9a9388ac gogs-48946299 gin-vue-admin go-test-bench parseable dart-pub gophish casdoor gitea-cut"
if [ "$SEL_HEAVY" = 1 ] && [ "$SEL_LIGHT" = 0 ]; then BENCHES=""; fi
[ "$SEL_LIGHT" = 1 ] && [ "$SEL_HEAVY" = 0 ] && BIGS=""
[ "$SEL_HEAVY" = 1 ] && [ -z "$BIGS" ] && { echo "--heavy needs --big NAME=URL@COMMIT" >&2; exit 64; }
BENCHES="$BENCHES $(big_names)"
if [ -n "$ONLY" ]; then BENCHES="$(echo "$ONLY" | tr ',' ' ')"; fi
if [ -n "$SKIP" ]; then
  keep=""
  for b in $BENCHES; do case ",$SKIP," in *",$b,"*) ;; *) keep="$keep $b" ;; esac; done
  BENCHES="$keep"
fi

# spec NAME [ROOT]: ROOT is the working tree (wt/<sha7>) of one side, default this script's; sets KIND (go|dart|rust|synth), URL, COMMIT, SUBDIR, RULES, DIR (the directory the analysis runs in), TRUTH
spec() {
  local R="${2:-$ROOT}"
  KIND=go; URL=""; COMMIT=""; SUBDIR=""; RULES=""; DIR=""; TRUTH=""; CLONE=""
  case "$1" in
    go-curated)    DIR="$R/bench/go-curated"; RULES="$DIR/rules.yaml" ;;
    zoo-go)        DIR="$R/bench/zoo-go"; RULES="$DIR/rules.yaml" ;;   # the zoo (docs/DESIGN-zoo.md): ./... reads the cells and mark; the harness is behind a build tag
    dart-curated)  KIND=dart; DIR="$R/bench/dart-curated"; RULES="$DIR/rules.yaml" ;;
    zoo-dart)      KIND=dart; DIR="$R/bench/zoo-dart/cells"; RULES="$DIR/rules.yaml" ;;   # the zoo of Dart: the cells and the declarations of zoo_mark; the harness (bench/zoo-dart/exec) is not read
    rust-curated)  KIND=rust; DIR="$R/bench/rust-curated"; RULES="$DIR/rules.yaml" ;;
    zoo-rust)      KIND=rust; DIR="$R/bench/zoo-rust"; RULES="$DIR/rules.yaml" ;;   # the zoo of Rust: the cells and the declarations of zoo-mark (no dependency); the harness (bench/zoo-rust/exec) is a workspace of its own and is not read
    go-synth-S)    KIND=synth; DIR="$WORK/synth/S"; RULES="$DIR/rules.yaml" ;;   # DIR is made per version in setup()
    go-scale)      KIND=synth; DIR="$WORK/synth/scale"; RULES="$DIR/rules.yaml" ;;   # bench/go-scale: all the shapes, made by the generator of this script's tree for both sides
    gogs-9a9388ac) URL=https://github.com/gogs/gogs; COMMIT=8a3b8198a; CLONE=gogs; RULES="$R/bench/gogs/rules.yaml"; TRUTH="$R/bench/gogs/truth-9a9388ac.json" ;;
    gogs-48946299) URL=https://github.com/gogs/gogs; COMMIT=ca59e76a5; CLONE=gogs-2; RULES="$R/bench/gogs/rules.yaml"; TRUTH="$R/bench/gogs/truth-48946299.json" ;;
    gin-vue-admin) URL=https://github.com/flipped-aurora/gin-vue-admin; COMMIT=7bc7c3051067; CLONE=gin-vue-admin; SUBDIR=server
                   RULES="$R/examples/rules/go-gin.yaml"; TRUTH="$R/bench/gin-vue-admin/truth.json" ;;
    go-test-bench) URL=https://github.com/Contrast-Security-OSS/go-test-bench; COMMIT=""; CLONE=go-test-bench
                   RULES="$R/examples/rules/go-gin.yaml"; TRUTH="$R/bench/go-test-bench/truth.json" ;;
    parseable)     KIND=rust; URL=https://github.com/parseablehq/parseable; COMMIT=5c20b8e477be162238037e202cea1869d657017b; CLONE=parseable
                   RULES="$R/bench/parseable/rules.yaml"; TRUTH="$R/bench/parseable/truth-5c20b8e4.json" ;;
    dart-pub)      KIND=dart; URL=https://github.com/dart-lang/pub; COMMIT=5e24336e71a05a96b558d496953ebed223f6e40a; CLONE=pub
                   RULES="$R/bench/dart-pub/rules.yaml"; TRUTH="$R/bench/dart-pub/truth-5e24336e.json" ;;
    gophish)       URL=https://github.com/gophish/gophish; COMMIT=27d13a0584731371033926064d750e5f4f620e7a; CLONE=gophish   # bench/wild/go: GHSA-9c9w-9pq7-f35h and GHSA-hvw3-p9px-gpc9, rules.yaml
                   RULES="$R/bench/wild/go/GHSA-9c9w-9pq7-f35h/rules.yaml"; TRUTH="$R/bench/wild/go/gophish-27d13a05.truth.json" ;;
    casdoor)       URL=https://github.com/casdoor/casdoor; COMMIT=7b0b426a76fd77b89817e0eafcccaed8d15b8cf4; CLONE=casdoor   # bench/wild/go: GHSA-9vm3-r8gq-cr6x
                   RULES="$R/bench/wild/go/GHSA-9vm3-r8gq-cr6x/rules.yaml"; TRUTH="$R/bench/wild/go/GHSA-9vm3-r8gq-cr6x/truth.json" ;;
    gitea-cut)     URL=https://github.com/go-gitea/gitea; COMMIT=v1.26.4; CLONE=gitea   # layer B: the cut-out of bench/gitea-cve-paths.md section 4 (7 packages, -no-deps: about 20 s, 1 GB), the three CVEs of Gitea
                   RULES="$R/bench/gitea/rules.yaml"; TRUTH="$R/bench/gitea-cve-truth.json" ;;
    *) local kv; for kv in $BIGS; do
         if [ "${kv%%=*}" = "$1" ]; then v="${kv#*=}"; URL="${v%@*}"; COMMIT="${v##*@}"; CLONE="$1"; fi
       done
       [ -n "$URL" ] || return 1 ;;
  esac
  [ -n "$CLONE" ] && DIR="$WORK/src/$CLONE${SUBDIR:+/$SUBDIR}"
  return 0
}

# --big NAME=URL@COMMIT: checked here, and an error stops the run (a wrong word was a silent skip or a URL that was not one)
for kv in $BIGS; do
  name="${kv%%=*}"; v="${kv#*=}"
  case "$kv" in *=*) ;; *) echo "--big: '$kv' has no '=': NAME=URL@COMMIT" >&2; exit 64 ;; esac
  case "$v" in *@*) ;; *) echo "--big: '$kv' has no '@': NAME=URL@COMMIT" >&2; exit 64 ;; esac
  url="${v%@*}"; commit="${v##*@}"
  case "$name" in ""|[!A-Za-z0-9]*|*[!A-Za-z0-9_.-]*) echo "--big: the name '$name' is not a short word (letters, digits, - _ .)" >&2; exit 64 ;; esac
  case "$url" in */*) ;; *) echo "--big: '$kv': '$url' is not a URL" >&2; exit 64 ;; esac
  case "$commit" in ""|*[!A-Za-z0-9._/-]*) echo "--big: '$kv': the commit '$commit' is not a full SHA or a tag" >&2; exit 64 ;; esac
  if ( BIGS=""; spec "$name" ); then echo "--big: the name '$name' is the one of a built-in benchmark (it would be ignored)" >&2; exit 64; fi
  n=0; for o in $BIGS; do [ "${o%%=*}" = "$name" ] && n=$((n+1)); done
  [ "$n" -le 1 ] || { echo "--big: the name '$name' is given twice" >&2; exit 64; }
done

# Rust: third-party code is read only under the sandbox of macOS (ADR-093, docs/rust-isolation.md): its build.rs and proc macros run
# during the extraction. Only these paths of this repository (our own code) may be read on this machine, with
# -unsafe-host-exec; everything else, a clone of someone else's project included, goes through `rivus extract -lang rust`,
# which is the sandbox. Without the sandbox (another system, or no sandbox-exec) the third-party benchmark FAILS; it never falls back to the machine.
OWN_RUST_PATHS="bench/zoo-rust bench/rust-curated extractor/rust/test"
NO_SANDBOX_MSG="Rust の第三者のコードは隔離の中でだけ読む。macOS のサンドボックス(sandbox-exec)が使えないので、このベンチを外す"
rust_is_own() { # dir root: 0 only when dir is, or is under, one of OWN_RUST_PATHS of root
  local d r p
  d="$(cd "$1" 2>/dev/null && pwd -P)" || return 1
  r="$(cd "$2" 2>/dev/null && pwd -P)" || return 1
  for p in $OWN_RUST_PATHS; do case "$d" in "$r/$p"|"$r/$p"/*) return 0 ;; esac; done
  return 1
}
rust_mode() { rust_is_own "$1" "$2" && echo host || echo sandbox; } # dir root
sandbox_ok() { command -v sandbox-exec > /dev/null 2>&1; } # whether the sandbox can be used (sandbox-exec is only on macOS)

# the flags (and the packages instead of ./...) that a benchmark is run with
flags_of() {
  case "$1" in
    gitea-cut) echo "-no-deps ./routers/web/repo ./services/release ./models/repo ./modules/storage ./routers/web/auth ./services/auth/source/oauth2 ./modules/auth/openid" ;;
  esac
}

memlimit_of() {
  local kv
  for kv in $MEMLIMITS; do [ "${kv%%=*}" = "$1" ] && ML="${kv#*=}"; done
  # a --big target is limited to 10GiB unless --gomemlimit NAME=V says otherwise (what --help says)
  if [ -z "$ML" ] && is_heavy "$1"; then ML=10GiB; fi
}

if [ "$DRY" = 1 ]; then
  echo "compare: base $BASE_REF = $BASE_SHA"
  echo "         head $HEAD_REF = $HEAD_SHA"
  echo "work dir: $WORK    results: $OUT (raw: $RES)"
  echo "1. git worktree add --detach $WORK/wt/$B7 $B7 ; go build -o $WORK/bin/$PAIR/rivus-base ./cmd/rivus   (skipped if the binary exists)"
  echo "   git worktree add --detach $WORK/wt/$H7 $H7 ; go build -o $WORK/bin/$PAIR/rivus-head ./cmd/rivus"
  echo "2. light tier: base and head at the same time when the free memory allows, no wait for other analyses (free memory >= 25%); the result of base is cached in ${CACHE_DIR:-$WORK/cache} unless --no-cache; heavy tier (--big): one at a time, no cache."
  echo "   the heavy tier (--big): before each run: wait for free memory >= 40% (swap <= 50% where it cannot be read) and no other rivus / rv-* / rivus-* process (at most $WAIT h)"
  NOSANDBOX_FAIL=0
  for b in $BENCHES; do
    if ! spec "$b"; then echo "   $b: unknown benchmark" >&2; continue; fi
    ML=""; memlimit_of "$b"
    case "$KIND" in dart) ex="extractor: ${RIVUS_DART:-dart} run wt/<sha7>/extractor/dart/bin/rivus_dart.dart";; rust)
      if [ "$(rust_mode "$DIR" "$ROOT")" = host ]; then ex="extractor: host (our own code: -unsafe-host-exec) ${RIVUS_RUST:-cargo build --locked in wt/<sha7>/extractor/rust}"
      else
        ex="extractor: sandbox (cargo build of wt/<sha7>/extractor/rust, then rivus extract -lang rust under the sandbox of macOS)"
        if ! sandbox_ok; then ex="$ex; the sandbox is not available: this benchmark FAILS ($NO_SANDBOX_MSG)"; NOSANDBOX_FAIL=1; fi
      fi;; *) ex="";; esac
    src="in-tree (wt/<sha7>/bench/$b)"; [ -n "$URL" ] && src="git clone $URL ; git checkout ${COMMIT:-<default branch>} ; ls go.mod ; go list ./..."
    [ "$KIND" = synth ] && src="go run ./bench/go-synth/gen -size S (in wt/<sha7>)"
    printf '   %-14s %-5s %s | rules: %s | GOMEMLIMIT=%s | -timeout %s %s\n' "$b" "$KIND" "$src" "${RULES:-built-in}" "${ML:-unset}" "$TIMEOUT" "$ex"
  done
  [ "${NOSANDBOX_FAIL:-0}" = 1 ] && echo "compare: $NO_SANDBOX_MSG" >&2
  echo "3. python3 bench/compare.py report: $OUT/summary.md, $OUT/details/<bench>.md; exit 1 if a finding was removed, 2 if a benchmark did not run"
    [ "${NOSANDBOX_FAIL:-0}" = 1 ] && exit 2
  exit 0
fi

mkdir -p "$WORK" "$RES" "$OUT/details" || exit 2
[ "$FIXED" = 1 ] && : > "$RES/fixed-inputs"   # bench/ledger.py reads it: the inputs are the ones of this tree

# one compare.sh at a time
# a light-only run has its own lock, so that it can run beside a --big one of the same work dir
LOCK="$WORK/lock.d"; [ "$SEL_LIGHT" = 1 ] && [ "$SEL_HEAVY" = 0 ] && LOCK="$WORK/lock-light.d"
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

# The inputs of one side are the ones of ITS OWN version (wt/<sha7>): the fixtures, the rules, the extractor.
# The IR is cached per (benchmark, extractor tree, target tree or commit, flags), never by the name alone:
# a fixture or an extractor fix that is in one version and not in the other makes another IR (R-41).
# wtree <path>: the hash of the files of that path in the WORKING tree of this script (committed or not, untracked ones too; empty when none).
# --fixed-inputs reads the working tree, so its keys (the cache of the base, the IR) are made from this and never from HEAD: an input that
# is changed and not committed is another input.
wtree() {
  local list
  list="$(cd "$ROOT" && git ls-files -co --exclude-standard -- "$1" | LC_ALL=C sort | while IFS= read -r f; do
    [ -f "$ROOT/$f" ] && printf '%s %s\n' "$f" "$(shasum -a 256 < "$ROOT/$f" | cut -d' ' -f1)"
  done)"
  [ -n "$list" ] && printf '%s\n' "$list" | shasum -a 256 | cut -c1-40
  return 0
}
tree() {   # tree <sha> <path>: the hash of that directory in that version (with --fixed-inputs: in the working tree, whatever the sha)
  if [ "$FIXED" = 1 ]; then wtree "$2"; else git -C "$ROOT" rev-parse -q --verify "$1:$2" 2>/dev/null; fi
}
h16() { shasum -a 256 | cut -c1-16; }
# the rivus of this script's tree that drives the sandbox (`rivus extract -lang rust`): a base that is older may not have it
DRIVER="$WORK/bin/driver/rivus"
ensure_driver() {
  [ -n "${DRIVER_OK:-}" ] && return 0
  mkdir -p "$WORK/bin/driver" && (cd "$ROOT" && go build -o "$DRIVER.$$" ./cmd/rivus) >&2 && mv "$DRIVER.$$" "$DRIVER" && DRIVER_OK=1
}
side_sha() { if [ "$1" = base ]; then echo "$BASE_SHA"; else echo "$HEAD_SHA"; fi; }

setup() { # name side: makes the copy and the IR of that version; sets DIR, RULES, IR (empty for Go); returns non-zero (and records why) if it cannot
  local b="$1" w="$2" sha s7 wt top tgt="" ext="" extkey="-" key irsha="-" rr
  sha="$(side_sha "$w")"; s7="${sha:0:7}"; wt="$WORK/wt/$s7"; IR=""
  if [ "$FIXED" = 1 ]; then sha="$(git -C "$ROOT" rev-parse HEAD)"; wt="$ROOT"; fi   # the inputs of this tree; s7 stays the engine's (the record of the run)
  spec "$b" "$wt" || { skip "$b" "unknown benchmark"; return 1; }
  PROV_INPUT="-"
  case "$b" in
    go-curated|dart-curated|rust-curated|zoo-go|zoo-dart|zoo-rust)
      tgt="$(tree "$sha" "bench/$b")"
      [ -n "$tgt" ] || { skip "$b" "bench/$b does not exist in $w ($s7)"; return 1; }
      PROV_INPUT="bench/$b tree ${tgt:0:12}" ;;
  esac
  if [ -n "$RULES" ] && [ "$KIND" != synth ] && [ ! -f "$RULES" ]; then   # a rules file that this old version does not have: the one of this script's tree
    [ -f "${RULES/#$wt/$ROOT}" ] && RULES="${RULES/#$wt/$ROOT}" || { skip "$b" "no rules file $RULES in $w ($s7)"; return 1; }
  fi
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
    tgt="$(git -C "$top" rev-parse HEAD)"; PROV_INPUT="$CLONE @ ${tgt:0:12}"
  fi
  case "$KIND" in
    go)
      if [ -n "$URL" ]; then
        [ -f "$DIR/go.mod" ] || { skip "$b" "no go.mod in $DIR"; return 1; }
        (cd "$DIR" && go list ./... > /dev/null 2>&1) || { skip "$b" "go list ./... failed in $DIR"; return 1; }
      fi ;;
    synth)
      if [ "$b" = go-scale ]; then   # the same input for both sides: the generator of this script's tree (a base before it has none)
        key="$(wtree bench/go-scale/gen | h16)"; DIR="$WORK/synth/scale-$key"; RULES="$DIR/rules.yaml"; PROV_INPUT="go-scale -shapes all (generator $key)"
        if [ ! -f "$DIR/rules.yaml" ]; then
          /bin/rm -rf "$DIR.tmp"; mkdir -p "$WORK/synth"
          (cd "$ROOT" && go run ./bench/go-scale/gen -shapes all -out "$DIR.tmp") >&2 && mv "$DIR.tmp" "$DIR" || { skip "$b" "the go-scale generator failed"; return 1; }
        fi
      else
      tgt="$(tree "$sha" bench/go-synth)$(tree "$sha" bench/synthplan)"; key="$(printf '%s' "$tgt" | h16)"
      DIR="$WORK/synth/S-$key"; RULES="$DIR/rules.yaml"; PROV_INPUT="go-synth -size S (generator $key)"
      if [ ! -f "$DIR/rules.yaml" ]; then
        /bin/rm -rf "$DIR.tmp"; mkdir -p "$WORK/synth"
        (cd "$wt" && go run ./bench/go-synth/gen -size S -out "$DIR.tmp") >&2 && mv "$DIR.tmp" "$DIR" || { skip "$b" "the generator failed in $w ($s7)"; return 1; }
      fi
      fi ;;
    dart)
      mkdir -p "$WORK/ir"
      local dartcmd="${RIVUS_DART:-dart}" nodeps="--no-deps" extdir="$wt/extractor/dart"
      case "$b" in dart-curated|zoo-dart) nodeps="" ;; esac
      [ -d "$extdir" ] || { skip "$b" "no extractor/dart in $w ($s7)"; return 1; }
      (cd "$DIR" && $dartcmd pub get) > /dev/null 2>&1 || { skip "$b" "dart pub get failed (RIVUS_DART=$dartcmd)"; return 1; }
      [ -d "$extdir/.dart_tool" ] || (cd "$extdir" && $dartcmd pub get) > /dev/null 2>&1
      ext="$(tree "$sha" extractor/dart)"; extkey="dart ${ext:0:12}"
      key="$(printf '%s|%s|%s|%s|%s' "$b" "$ext" "$dartcmd" "$tgt" "$nodeps" | h16)"
      IR="$WORK/ir/$b.$key.jsonl"
      if [ ! -s "$IR" ]; then
        say "$b: making the IR with the extractor of $w ($s7)"
        (cd "$extdir" && $dartcmd run bin/rivus_dart.dart $nodeps "$DIR" > "$IR.tmp") 2>"$WORK/ir/$b.$key.err" || { /bin/rm -f "$IR.tmp"; skip "$b" "the Dart extractor failed (see $WORK/ir/$b.$key.err)"; return 1; }
        mv "$IR.tmp" "$IR"
      fi ;;
    rust)
      mkdir -p "$WORK/ir"
      local mode
      mode="$(rust_mode "$DIR" "$wt")"
      if [ "$mode" = host ]; then   # our own code (OWN_RUST_PATHS)
        if [ -n "${RIVUS_RUST:-}" ]; then   # an extractor named by the caller is used for both sides
          rr="$RIVUS_RUST"; [ -x "$rr" ] || { skip "$b" "RIVUS_RUST=$rr is not executable"; return 1; }
          ext="bin-$(shasum -a 256 "$rr" | h16)"
        else
          ext="$(tree "$sha" extractor/rust)"; [ -n "$ext" ] || { skip "$b" "no extractor/rust in $w ($s7)"; return 1; }
          rr="$WORK/bin/rust-extractor/$ext/rivus-rust"
          if [ ! -x "$rr" ]; then   # one binary per extractor tree; the cargo target dir is shared, so the dependencies are built once
            say "building the Rust extractor of $w ($s7, tree ${ext:0:12})"
            (cd "$wt/extractor/rust" && CARGO_TARGET_DIR="$WORK/cargo-target" cargo build --locked) > "$WORK/ir/rust-extractor-$ext.log" 2>&1 \
              || { skip "$b" "cargo build --locked failed in $w ($s7) (see $WORK/ir/rust-extractor-$ext.log)"; return 1; }
            mkdir -p "$WORK/bin/rust-extractor/$ext" && cp "$WORK/cargo-target/debug/rivus-rust" "$rr" || { skip "$b" "no rivus-rust after cargo build"; return 1; }
          fi
        fi
        extkey="rust ${ext:0:12}"
      else   # someone else's code: under the sandbox, or nothing
        sandbox_ok || { skip "$b" "$NO_SANDBOX_MSG"; return 1; }
        ext="$(tree "$sha" extractor/rust)"; [ -n "$ext" ] || { skip "$b" "no extractor/rust in $w ($s7)"; return 1; }
        rr="$WORK/bin/rust-extractor/$ext/rivus-rust"   # the extractor of THIS version: one binary per extractor tree
        if [ ! -x "$rr" ]; then
          say "building the Rust extractor of $w ($s7, tree ${ext:0:12})"
          (cd "$wt/extractor/rust" && CARGO_TARGET_DIR="$WORK/cargo-target" cargo build --locked) > "$WORK/ir/rust-extractor-$ext.log" 2>&1 \
            || { skip "$b" "cargo build --locked failed in $w ($s7) (see $WORK/ir/rust-extractor-$ext.log)"; return 1; }
          mkdir -p "$WORK/bin/rust-extractor/$ext" && cp "$WORK/cargo-target/debug/rivus-rust" "$rr" || { skip "$b" "no rivus-rust after cargo build"; return 1; }
        fi
        ensure_driver || { skip "$b" "go build of the rivus that drives the sandbox failed"; return 1; }
        extkey="rust-sandbox ${ext:0:12}"
      fi
      key="$(printf '%s|%s|%s|%s' "$b" "$ext" "$tgt" "$mode" | h16)"
      IR="$WORK/ir/$b.$key.jsonl"
      if [ ! -s "$IR" ]; then
        say "$b: making the IR with the extractor of $w ($s7), $mode"
        if [ "$mode" = sandbox ]; then
local xargs=(-lang rust -C "$DIR" -rust-extractor "$rr")
          if [ "$b" = parseable ]; then
            mkdir -p "$WORK/parseable-assets"   # build.rs would fetch the UI from the network without it
            xargs+=(-no-deps -isolation-env "LOCAL_ASSETS_PATH=$WORK/parseable-assets" -isolation-read "$WORK/parseable-assets")
          fi
          # RIVUS_UNSAFE_HOST_EXEC is removed: there is no way to the host from here
          env -u RIVUS_UNSAFE_HOST_EXEC "$DRIVER" extract "${xargs[@]}" > "$IR.tmp" 2>"$WORK/ir/$b.$key.err" \
            || { /bin/rm -f "$IR.tmp"; skip "$b" "the Rust extractor failed under the sandbox (see $WORK/ir/$b.$key.err)"; return 1; }
        else
          (cd "$wt" && "$rr" "$DIR" > "$IR.tmp") 2>"$WORK/ir/$b.$key.err" || { /bin/rm -f "$IR.tmp"; skip "$b" "the Rust extractor failed (see $WORK/ir/$b.$key.err)"; return 1; }
        fi
        mv "$IR.tmp" "$IR"
      fi ;;
  esac
  if [ -n "$IR" ]; then   # the hash is taken with this version's path replaced, so the same input in two worktrees has the same hash
    [ -s "$IR.sha256" ] || python3 "$PY" irsum "$IR" "$wt" > "$IR.sha256"
    irsha="$(cat "$IR.sha256")"
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "$PROV_INPUT" "$extkey" "$irsha" "$s7" "$RULES" > "$RES/$b.$w.prov"
  return 0
}

# --- 3. the runs: bench by bench; the light tier runs base and head at the same time when the memory allows, the heavy one one at a time ---
CACHE_DIR="${CACHE_DIR:-$WORK/cache}"

# cache_tree: the state of the target of the base side, as JSON (a part of the key of the cache; never the name of a version)
cache_tree() { # bench: uses DIR, KIND, PROV_INPUT of the last setup; prints JSON
  local b="$1" ir irsha dirty=false top gosum="" gov="" genv="" wt="$WORK/wt/${BASE_SHA:0:7}"
  irsha="$(cut -f3 "$RES/$b.base.prov")"
  [ -n "$(git -C "$wt" status --porcelain 2>/dev/null)" ] && dirty=true
  [ -n "$URL" ] && [ -n "$(git -C "$WORK/src/$CLONE" status --porcelain 2>/dev/null)" ] && dirty=true
  case "$KIND" in
    go|synth)
      [ -f "$DIR/go.sum" ] && gosum="$(shasum -a 256 "$DIR/go.sum" | cut -d' ' -f1)"
      gov="$(go version)"; genv="$(go env GOOS GOARCH GOFLAGS | tr '\n' ' ')" ;;
  esac
  python3 -c 'import json,sys; k=["input","ir_sha256","dirty","go_sum_sha256","go_version","go_env"]; d=dict(zip(k,sys.argv[1:])); d["dirty"]=(d["dirty"]=="true"); print(json.dumps(d))' \
    "$(cut -f1 "$RES/$b.base.prov")" "$irsha" "$dirty" "$gosum" "$gov" "$genv"
}

run_side() { # bench side tier: runs one analysis of one side with the variables DIR_<side>, IR_<side>, RULES_<side>
  local b="$1" w="$2" tier="$3" DIR IR RULES args tree
  eval "DIR=\$DIR_$w; IR=\$IR_$w; RULES=\$RULES_$w"
  say "$b: $w"
  args=(run -rivus "$BIN/rivus-$w" -dir "$DIR" -out "$RES/$b.$w" -timeout "$TIMEOUT" -wait "$WAIT" -gomemlimit "$ML")
  [ -n "$RULES" ] && args+=(-rules "$RULES")
  [ -n "$IR" ] && args+=(-ir "$IR")
  [ -n "$(flags_of "$b")" ] && args+=(-flags "$(flags_of "$b")")
  [ "$NOGUARD" = 1 ] && args+=(-no-guard)
  [ "$tier" = light ] && args+=(-light)
  if [ "$tier" = light ] && [ "$w" = base ] && [ "$NOCACHE" = 0 ]; then args+=(-cache-dir "$CACHE_DIR" -cache-tree "$CACHE_TREE"); fi
  python3 "$PY" "${args[@]}" ${rf[@]+"${rf[@]}"} || true
}

run_bench() { # bench
  local b="$1" ok=1 w tier=light pid f
  is_heavy "$b" && tier=heavy
  for w in base head; do
    setup "$b" "$w" || { ok=0; break; }
    eval "DIR_$w=\$DIR; IR_$w=\$IR; RULES_$w=\$RULES"
    if [ "$w" = base ]; then CACHE_TREE="$(cache_tree "$b")"; fi
  done
  [ "$ok" = 1 ] || return 0
  ML=""; memlimit_of "$b"
  rf=(); [ "$RESUME" = 1 ] && rf=(-resume)
  if [ "$tier" = light ]; then
    run_side "$b" base light & pid=$!
    # head at the same time when the memory allows (the 3 GB runs: two at a time), else after base
    for _ in 1 2 3 4 5 6; do sleep 2; kill -0 "$pid" 2>/dev/null || break; done
    f="$(python3 "$PY" freemem)"
    if kill -0 "$pid" 2>/dev/null && { [ -z "$f" ] || [ "$f" -lt 35 ]; }; then wait "$pid"; fi
    run_side "$b" head light
    wait "$pid" 2>/dev/null
  else
    run_side "$b" base heavy
    run_side "$b" head heavy
  fi
  spec "$b"   # the truth is the one of this script's tree
  if [ -n "$TRUTH" ]; then
    for w in base head; do
      [ -s "$RES/$b.$w.json" ] || continue
      if python3 "$ROOT/bench/score.py" "$TRUTH" "$RES/$b.$w.json" > "$RES/$b.$w.score" 2> "$RES/$b.$w.score.err"; then
        grep -E '^(TP [0-9]|uncertainties:|source-only entries:)' "$RES/$b.$w.score" | sed "s/^/$b $w: /" >> "$RES/scores.txt"
      else
        echo "$b $w: SCORING FAILED: $(tail -n 1 "$RES/$b.$w.score.err")" >> "$RES/scores.txt"
      fi
    done
  fi
}

ALL=""
for b in $BENCHES; do
  ALL="$ALL,$b"
  run_bench "$b"
done

# --- 4. the table ---
python3 "$PY" report -results "$RES" -base "$([ "$BASE_REF" = "$B7" ] && echo "$B7" || echo "$BASE_REF ($B7)")" -head "$([ "$HEAD_REF" = "$H7" ] && echo "$H7" || echo "$HEAD_REF ($H7)")" -out "$OUT" -benches "${ALL#,}" -skipped "$SKIPPED"
rc=$?
if [ -s "$RES/scores.txt" ]; then
  { echo; echo "Scores against the truth files (bench/score.py; the labelled sites):"; echo; sed 's/^/    /' "$RES/scores.txt"; } | tee -a "$OUT/summary.md"
fi
say "summary: $OUT/summary.md   exit $rc"
exit $rc
