# Benchmarks

Ground truth for open-source targets, and a scorer. The targets are not vendored: check them out
next to this repository, run `rivus analyze`, and score the result.

```sh
git clone https://github.com/Contrast-Security-OSS/go-test-bench && cd go-test-bench
rivus analyze -rules /path/to/rivus/examples/rules/go-gin.yaml ./... > result.json   # the rules the target needs
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/go-test-bench/truth.json result.json
```

A finding is matched with a labelled sink site by (file, line, category). The labels were made by
reading the code (the targets have no answer key), and the judgement calls are in each entry's
`note`: the truth file is a claim, and anyone can read and dispute it.

Results are recorded in `docs/DECISIONS.md` (ADR-043).

## Benchmarks that are in this repository

The flows of the ground truths here (Gitea's three CVEs, gin-vue-admin, Gogs, dart-pub, parseable) are
also kept as **small fixtures of their shape** in the curated projects (the CVE shapes, T-5; see the
last section of [`MISSES.md`](MISSES.md)), so that `go test ./...` notices when one of these flows is
lost without running the real project: `go-curated/gitea`, `go-curated/gva`, `go-curated/gogs`,
`dart-curated/lib/pub_tar.dart`, `rust-curated/src/parseable.rs`.

The targets above are real projects: they are big, slow to analyse, and have no answer key but the
labels made by hand. For the work on the engine there are also small benchmarks of our own, made
for each language (Go, Dart, Rust):

| directory | what it is | what it answers | time |
| --- | --- | --- | --- |
| `go-curated/` | a project in the shape of Gitea, gogs, gin-vue-admin, ech0, with the flows to find and the ones not to find written in the source | did the accuracy change (a flow lost, a false one gained)? | seconds, in `go test ./...` |
| `dart-curated/`, `rust-curated/` | a package / a crate of the shapes that matter for closures and callbacks (helpers that take a closure, two callers; captures; relays; tear-offs and function pointers; errors), in the same marks, with what is not found yet (`@miss`) and what is reported wrongly (`@fp`, and in Rust `@noisy(N)`) recorded: the baseline for a change of how closures and callbacks are summarised | the same, for closures and callbacks | seconds, in `go test ./...` with `RIVUS_DART` / `RIVUS_RUST` set |
| `go-synth/` | a generator of Go code of a given shape (depth, SCCs, interfaces, fields, ...), with a script (`measure.py`) that measures time, memory and the result | did the time or the memory change, and is the result the same? | S seconds, M minutes |
| `dart-synth/`, `rust-synth/` | generators of a Dart package / a Rust crate (depth, SCCs, closures given to shared helpers, relays, closures in fields, captures); the plan is `synthplan/`; `go-synth/measure.py -lang dart` or `-lang rust` runs the extractor once and measures `rivus analyze -ir` | the same | S under a second, M and L seconds |

When the core of the engine (`internal/sg`, `internal/sites`, `internal/taint`) is touched: run
`go test ./...` (this includes `go-curated`; with `RIVUS_DART` and `RIVUS_RUST` set, `dart-curated` and
`rust-curated` too), then the S and M of `go-synth`, `dart-synth` and `rust-synth` before and after with the
same binary built from each state, and check that the result is identical (or that every changed
finding is understood). A real project (Gitea) is for a milestone only: it takes ten to sixteen
minutes and 5 to 10 GB. See `go-synth/README.md` for how, and `docs/DECISIONS.md` (ADR-046) for why.

Two more things are held by the three curated tests. Each `@miss` says whether the engine left a
trace of the flow it lost: `@miss(silent)` (nothing in the functions of the source and the sink; 42 of the
50: Dart 27, Rust 13, Go 2) or `@miss(<kind>)` (8: Dart 7, Go 1); and the findings whose path has no step in
the function of their source are counted against a ceiling (BENCH-2; 0 in the three, 77 of 88 in
`TestDogfood`, 1 each in three samples of `paramshare`). See `MISSES.md`.

```sh
export RIVUS_DART="mise exec dart@3.13.6 -- dart" RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust
go test ./cmd/rivus -run Curated -v        # the three accuracy benchmarks: counts in the log
```

## The share of silent misses

`@miss(silent) / @miss`: of the flows the engine loses, how many it loses **without a word**. The
definition is the one of `rivus resolve`'s `absent_now`: the functions of the sites on the source
line and the sink line hold no uncertainty at all. Not counted as a trace: an uncertainty that has no
function (a whole-run one, such as Rust's `depth_limit` / `path_limit`), `numeric_stop`, and a
`dynamic_dispatch` with exactly one candidate (it is raised at every method call and does not say that
a callback was lost). Uncertainties of a helper on the way (not in the function of the source or the
sink) are not counted either.

Each curated test logs one line (`go test ./cmd/rivus -run Curated -v`, with `RIVUS_DART` and
`RIVUS_RUST` set for Dart and Rust):

```
go-curated silent misses: 12 / 13 (92%)
dart-curated silent misses: 33 / 43 (77%)
rust-curated silent misses: 15 / 16 (94%)
```

| | `@miss` | `@miss(silent)` | `@miss(<kind>)` | silent share |
| --- | --- | --- | --- | --- |
| Dart | 43 | 33 | 10 | 77% |
| Rust | 16 | 15 | 1 | 94% |
| Go | 13 | 12 | 1 | 92% |
| all | 72 | 60 | 12 | 83% |

(2026-11-21, after the fixtures of C-7 (`docs/DESIGN-capture.md`); the counts are the ones the tests check, so they match the annotations. It was 42 / 50 = 84% before the closure-capture fixtures, and 53 / 64 = 83% with the first ones of them.) **The share did not move because the engine got worse: the fixtures that were added record misses that were not seen before** (a captured value lost in a helper's callback, Go 10, Dart 9, Rust 3 of the 22 added); the engine is the same.

What it means: it measures how well the discipline of leaving a trace is kept, i.e. how immature the
engine is where it loses a flow. It stands next to the share of `attributable_to: rivus` among the
reasons a flow is not found: that one says whose fault a miss is, this one says whether the engine told
anyone. It is a log, not a ceiling: it is the number to watch go down when a fix adds traces (DART-15,
CORE-D, C-7). A trace that appears makes the `@miss(silent)` line fail as an improvement (rewrite it as
`@miss(<kind>)`), so the number cannot move without someone seeing it. A miss that is fixed leaves both
the numerator and the denominator, so it does not count as an improvement of this share.
