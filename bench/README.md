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

## Changing the accuracy of the core: the procedure

**A change of the accuracy of the core (`internal/sg`, `internal/sites`, `internal/taint`, and what
they read: summaries, the call resolution, the sources) is pushed only when the benchmarks of the
real code have been run before and after it, and every difference in their findings has been
looked at. If they could not be run (a checkout without `go.mod`, a target that does not build, no
Dart or Rust toolchain), do not push**: say so and wait. `go test ./...` and the curated projects are
not enough, because the shapes in them are the ones that were already known (the regression of
ADR-048, a540f30 to c44bfdd, was found only by the findings of gogs and gin-vue-admin).

**In one command: `bench/compare.sh <base> <head>`** (for example `bench/compare.sh c44bfdd origin/main`)
does steps 1, 4 and the comparison of 5 for you: it builds both states in worktrees, makes the copies of
the targets of the table below (`git clone` and `git checkout`, then `ls go.mod` and `go list ./...`; a
copy that cannot be made is listed as NOT RUN), runs **one benchmark at a time, base then head**, and
writes `summary.md` (one row per benchmark; a removed finding is marked `!!` at the top) and
`details/<bench>.md` (every added, removed, and changed finding, one per line: reachability, the
representative path, `other_sources`, the uncertainties by kind and by `depth_limit` reason, `stats`,
peak RSS; a run that did not converge is said so). Exit status: 0 nothing lost, 1 a finding was
removed, 2 something did not run. Before each run it waits until swap use is 50% or less and no other
`rivus` / `rv-*` / `rivus-*` process runs (`--wait HOURS`); `GOMEMLIMIT` is set per benchmark
(`--gomemlimit NAME=VAL`; gogs 5GiB, Gitea 10GiB); `-timeout` is the same for both. For a night:
`nohup bench/compare.sh --resume <base> <head> > compare.log 2>&1 &` (each result is saved per
benchmark; `--resume` skips the ones that are done). Gitea (`--with-gitea`) and ech0 (`--with-ech0`) are
off by default. `--dry-run` prints the plan; `--help` the options. The classification of step 5, and
step 2 (`go test ./...`) and step 3 (`go-synth`), are still yours. The script does not score against
the truth files except for printing `score.py`'s last lines.

**What each side is analysed with (R-41).** Base and head are each analysed with the inputs of their own version, taken
from `<work>/wt/<sha7>`: the fixtures (`bench/*-curated`, the `go-synth` generator), the rules files, and the extractor
(`extractor/dart`, `extractor/rust`). A fixture added in head is therefore seen only by head (it shows as `+1` in the row
of that benchmark), and an extractor fix changes the IR of head only. The IR is cached **per version**, in
`<work>/ir/<bench>.<key>.jsonl`, where the key is a hash of the benchmark, the tree hash of the extractor
(`git rev-parse <sha>:extractor/<lang>`), the tree hash of the target (`<sha>:bench/<name>`; for a clone, its commit), the flags
and `RIVUS_DART`. The same input is never extracted twice (parseable takes about 95 s, dart-pub about as long), and an input
that changed is never read from the old file. The Rust extractor of each version is built once (`cargo build --locked`, one
shared `CARGO_TARGET_DIR` in the work dir, the binary kept in `<work>/bin/rust-extractor/<tree>/`); `RIVUS_RUST` (a
binary) or `RIVUS_DART` (a command), when set, name the tool for both sides. A rules file that an old base does not have is
taken from the tree of the script, and the other way round is not done. `bench/compare.sh --dry-run` shows the plan.

**How to read `summary.md` (R-40, R-41).** Its top lines come first, one per condition, and a line that starts with `!! INVALID`
makes the exit status 2 (the run is not a basis for a comparison; a removed finding is still exit status 1):

| line | meaning |
| --- | --- |
| `head did not converge (base did)` | head stopped by the pass limit; a lost finding would not show |
| `base / head timed out` | the clock stopped the run: the result depends on the load of the machine |
| `stats.functions N -> M (x%)` with `!! INVALID` | 50% or more: not the same program (an extractor that lost a package, a checkout that is not whole) |
| `warning: ... stats.functions` | 10% to 50%: look at why, no exit status |
| `NOISE: <bench> +N (P%): classification needed` | more than 20% of the base findings and more than 5 were ADDED at once (exit status 2, apart from the 1 of a removal). A benchmark without a truth cannot say that they are wrong: read details/<bench>.md one by one and write the result in the commit message or the review. There is no flag that silences it |
| `no finding in base and none in head` | the input or the rules are empty (a benchmark with a truth has findings) |

Under the table, "Inputs of each side" has, per benchmark, the input (the tree hash of the fixtures, or the commit of the
clone), the tree hash of the extractor, and the sha256 of the IR of base and head (with the path of the worktree replaced,
so the same input in two worktrees has the same hash); `IR: same` says that only the binary differs, `DIFFERENT` that the
extractor or the fixtures changed too. Gitea's base that does not converge is the one `NOT CONVERGED` note, not an `INVALID`.

The synthetic tests of this report (exit statuses) are `python3 bench/test_compare.py`.

**Tiers (`--light`, `--heavy`).** The tiers are measured, not named. Light: one run of 60 seconds or less and 6 GB or less
(curated three, go-synth-S, `go-scale`, gogs x 2, gin-vue-admin, go-test-bench, parseable, dart-pub). Heavy: Gitea and ech0.
Within the light tier, base and head of a benchmark run at the same time when the free memory allows it (checked a few seconds after
base started: 35% free or more), and a light run waits only for free memory (25%), not for other analyses; a light-only run has its own
lock (`lock-light.d`), so it can run beside a heavy one of the same work dir. The time and the peak RSS of a light run are therefore
measured beside another run: read them as a guide. Heavy: one at a time, waits for a quiet machine, and never uses the cache; it is
the final check of a big change (`--heavy`, or `--with-gitea` after the light tier). The real codes (gogs etc.) stay in the light tier: they
are the net that found unknown regressions.

**The cache of base (`<work>/cache`, `--no-cache`).** The result of BASE in the light tier is kept and reused by the next comparison with
the same base, because the analysis is deterministic (`TestDeterministic*`). The key is the sha256 of the `rivus` binary, the state of
the target (Go: the commit or tree hash, a clean tree, the sha256 of `go.sum`, `go version`, GOOS / GOARCH / GOFLAGS; Dart and Rust: the
sha256 of the IR, i.e. the extractor and the target of R-41), the content of the rules file, `-timeout`, the flags, GOMEMLIMIT, GOMAXPROCS,
GOGC: never the name of a version. A dirty tree is not cached. A run with `timed_out`, `converged: false`, `func_timeouts` or
`budget_exhausted` is not kept (a clock decided). The material of the key is kept with the result and compared when it is read. Until
the number of functions is stable between runs (B-30), a cached row can differ in `stats` from a fresh one: `summary.md` says
`<bench>: base: cached <date>`, and the time and the peak RSS are those of the run that made it.

**Scores (`scores.txt`, R-45).** `score.py` matches a truth entry by its sink (`file`, `line`) or, for an entry with
`source_func`, by the source function; if the entry also has `sink_file` and `sink_line`, by the pair (source function,
sink), which a finding from that function to another sink does not satisfy. An entry with a `source_func` only does not look
at the sink, so a run that over-reports scores as well as one that does not: the score prints how many entries are of that
kind (`source-only entries: N of M`). The two gogs truth files are all of that kind today. A label other than `vuln` or
`safe`, or an entry without a category, is an error; a score that could not be made is written in `scores.txt`, not dropped.

1. Build the binary of each state (the commit before, and the working tree), from the same toolchain:
   `git stash` or `git worktree add <dir> <before>`, then `go build -o <dir>/rivus-before ./cmd/rivus`
   and `go build -o <dir>/rivus-after ./cmd/rivus`. Keep them; every step below runs both.
2. `go test ./...` with `RIVUS_DART` and `RIVUS_RUST` set (see above): the three curated tests
   (`go-curated`, `dart-curated`, `rust-curated`) must pass with the counts of `@miss`, `@fp`,
   `@noisy` that are annotated. A count that moved is a finding to be classified, like those below.
3. `go-synth`: `python3 bench/go-synth/measure.py run -rivus <bin> -size S` (and `-size M`), once per
   binary, then `measure.py compare before.json after.json`. For a change that is not meant to
   change the result it must say IDENTICAL; for one that is, step 5 applies. Do the same with
   `-lang dart` and `-lang rust` when the change can touch them.
4. The real code: run both binaries on **gogs** (both truths), **gin-vue-admin**, **ech0**,
   **go-test-bench**, **parseable** and **dart-pub**, with the rules of each (table below), save
   the two reports, and compare the **set of finding IDs** (`measure.py` prints a hash of it; for a
   difference, diff `findings[].id`) and the score (`score.py`, the truth files here).
5. **Classify every finding that appears or disappears, one by one**: read the source, the
   path and the sink, and write down whether it is real, a false positive that went away, a false
   positive that came, or a flow that was lost. **A single flow that was lost (a miss that the
   before-binary did not have) means: do not push.** Fix the change, or change what it does, and run
   again. A false positive that goes away is a gain only after it has been read as one.
6. Put the numbers and the classification in the commit message or the ADR (`docs/DECISIONS.md`:
   ADR-048 has the form), including what was not run and why.

### How to make the copies

Make each copy with **`git clone` and `git checkout <commit>`**, not by copying a directory
with filters, and check that it is whole before measuring: `ls go.mod` (for gin-vue-admin, in
`server/`) and `go list ./... > /dev/null` for Go; `dart pub get` for Dart; for Rust see
`parseable/README.md`. A copy without `go.mod` makes `rivus analyze` fail or load nothing, and a run
on it is not a run: it counts as "not run" (above). Keep the clones outside the repository (the
scratchpad of the session, or `~/ghq`), one directory per target, so that the two binaries read the
same files.

| target | where from | commit | rules | scored with |
| --- | --- | --- | --- | --- |
| gogs (CVE-2024-55947) | <https://github.com/gogs/gogs> | `8a3b8198a` | `bench/gogs/rules.yaml` | `bench/gogs/truth-9a9388ac.json` |
| gogs (CVE-2026-23633) | <https://github.com/gogs/gogs> | `ca59e76a5` | `bench/gogs/rules.yaml` | `bench/gogs/truth-48946299.json` |
| gin-vue-admin | <https://github.com/flipped-aurora/gin-vue-admin>, module `server/` | `7bc7c3051067` (the parent of the fix `b1b7427c6ea6`) | `examples/rules/go-gin.yaml` (the gin model; the file that was used is not recorded in the repository: not confirmed) | `bench/gin-vue-admin/truth.json` |
| go-test-bench | <https://github.com/Contrast-Security-OSS/go-test-bench> | not recorded (the `truth.json` says "the commit in `commit`", and has no such field): the default branch at the time, unrecorded | `examples/rules/go-gin.yaml` (above) | `bench/go-test-bench/truth.json` |
| ech0 | https://github.com/lin-snow/Ech0 (`bench/compare.sh` clones it) | `cecc2c19` (the parent of the fix `091d26d2`) | built-in rules only in `compare.sh`; the measurements of ADR-047 used `examples/rules/go-gin.yaml` on the head of 2026-11-21 | not scored |
| parseable | <https://github.com/parseablehq/parseable> | `5c20b8e477be162238037e202cea1869d657017b` | `bench/parseable/rules.yaml` | `bench/parseable/truth-5c20b8e4.json` |
| dart-pub | <https://github.com/dart-lang/pub> | `5e24336e71a05a96b558d496953ebed223f6e40a` | `bench/dart-pub/rules.yaml` | `bench/dart-pub/truth-5e24336e.json` |
| Gitea (milestones only) | <https://github.com/go-gitea/gitea> | tag `v1.26.4` | `bench/gitea/rules.yaml` | `bench/gitea/truth-v1.26.4.json` |

How to run each is in the README of the target where there is one (`dart-pub/`, `parseable/`; the top of
this file for the Go ones). "Not recorded" is a gap to be filled the next time someone has the
checkout: write the commit here when it is known.

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
trace of the flow it lost: `@miss(silent)` (nothing in the functions of the source and the sink; 51 of the
78: Dart 31, Rust 15, Go 5) or `@miss(<kind>)` (27: Dart 16, Rust 1, Go 10); and the findings whose path has no step in
the function of their source are counted against a ceiling (BENCH-2; 0 in the three, 0 of 39 in
`TestDogfood` and `paramshare` too: CORE-B was fixed, ADR-049). See `MISSES.md`.

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
go-curated silent misses: 5 / 15 (33%)
dart-curated silent misses: 31 / 47 (66%)
rust-curated silent misses: 15 / 16 (94%)
```

| | `@miss` | `@miss(silent)` | `@miss(<kind>)` | silent share |
| --- | --- | --- | --- | --- |
| Dart | 47 | 31 | 16 | 66% |
| Rust | 16 | 15 | 1 | 94% |
| Go | 15 | 5 | 10 | 33% |
| all | 78 | 51 | 27 | 65% |

(2026-11-21, **ADR-053 (DART-15): 66 / 78 = 85% before, 51 / 78 = 65% now**: 15 of the misses of a captured value (Go 9, Dart 6) now have a `depth_limit` (`capture_unbound`) in the function that made the closure; the engine finds the same flows, and says what it does not know. Rust: none, see ADR-053. Before it: `alias-ptr` (`go-curated/api/alias.go`: one object reached through two pointers, a limit of the alias analysis, Go +1 silent; 77 / 65 = 84% before it); ADR-050 added `limits.dart` (Dart +2 misses, both told in the helper by `depth_limit`: `candidate_limit` / `caller_depth_limit`; this share does not count a trace in a helper, so they stay silent); after the fixtures of C-7 (`docs/DESIGN-capture.md`); the counts are the ones the tests check, so they match the annotations. It was 42 / 50 = 84% before the closure-capture fixtures, and 53 / 64 = 83% with the first ones of them.) **The share did not move because the engine got worse: the fixtures that were added record misses that were not seen before** (a captured value lost in a helper's callback, Go 11, Dart 11, Rust 3 of the 25 added); the engine is the same.

What it means: it measures how well the discipline of leaving a trace is kept, i.e. how immature the
engine is where it loses a flow. It stands next to the share of `attributable_to: rivus` among the
reasons a flow is not found: that one says whose fault a miss is, this one says whether the engine told
anyone. It is a log, not a ceiling: it is the number to watch go down when a fix adds traces (DART-15,
CORE-D, C-7). A trace that appears makes the `@miss(silent)` line fail as an improvement (rewrite it as
`@miss(<kind>)`), so the number cannot move without someone seeing it. A miss that is fixed leaves both
the numerator and the denominator, so it does not count as an improvement of this share.

**ech0 does not finish (2026-11-21).** It reads the dependencies (the AWS SDK, protobuf: the default of `rivus analyze ./...`), and
with `examples/rules/go-gin.yaml` on the head of the repository it does not converge in 900 seconds, with or without the
callouts of ADR-047 (2 passes, 15 GB, 48 findings; the summaries of the AWS middleware grow from 36 to 2,200 elements in a few
passes, almost all of them new, not coming back). It is a measure of the growth of the summaries, not a baseline to compare findings
with: a run that does not converge is cut where it was.

**Gitea: use the tag.** The truth file `bench/gitea/truth-v1.26.4.json` is for the tag `v1.26.4` (the three CVEs are in that
code, and fixed in later releases). `bench/compare.sh --with-gitea` checks out the tag in its own copy. A copy of the
repository that was cloned some other time is the head of `main` of that day, not the tag: a measurement on it (the time, the
passes, the recall of the three CVEs) is of other code, and has to be taken again on the tag. Check with
`git describe --tags` (or `git rev-parse HEAD` against the tag) before a measurement is written down.
