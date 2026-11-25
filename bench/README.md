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

Results are recorded in `docs/adr/043-accuracy-benchmark-go-test-bench.md` (ADR-043).

**Where things are.** This file has: the procedure for a change of the core and `compare.sh` (below), the targets and where to fetch them,
the table of the directories ("Benchmarks that are in this repository"), the share of silent misses, and the scale profiles
(`go-scale/`). Records of measurements: [`LEDGER.md`](LEDGER.md), [`BACKTEST.md`](BACKTEST.md), [`explore/LOG.md`](explore/LOG.md);
the classification of the `@miss` / `@fp` / `@noisy` of the curated projects: [`MISSES.md`](MISSES.md) (a snapshot, see its header).

## When a function may be `pure`

Moved to [`docs/MODELING.md`](../docs/MODELING.md) (the guide for whoever writes a model, by hand or with an AI). The tests that
hold the rule are `TestPureFunctionsThatReturnDataAreReviewed` (`internal/model/builtin`) and `TestZooPureGo`
(`cmd/rivus/zoo_pure_test.go`, `bench/zoo-go/pure`); the `pure` rows of Go that the run does not call (250 of the 272 on 2026-11-21, by the header of that file; a count that moves with the model) are listed with the
reason of each in `bench/zoo-go/pure/unchecked.txt`.

## Changing the accuracy of the core: the procedure

**A change of the accuracy of the core (`internal/sg`, `internal/sites`, `internal/taint`, and what
they read: summaries, the call resolution, the sources) is pushed only when the benchmarks of the
real code have been run before and after it, and every difference in their findings has been
looked at. If they could not be run (a checkout without `go.mod`, a target that does not build, no
Dart or Rust toolchain), do not push**: say so and wait. `go test ./...` and the curated projects are
not enough, because the shapes in them are the ones that were already known (the regression of
ADR-048, 623b620 to c9aa412, was found only by the findings of gogs and gin-vue-admin).

**In one command: `bench/compare.sh <base> <head>`** (for example `bench/compare.sh c9aa412 origin/main`)
does steps 1, 4 and the comparison of 5 for you: it builds both states in worktrees, makes the copies of
the targets of the table below (`git clone` and `git checkout`, then `ls go.mod` and `go list ./...`; a
copy that cannot be made is listed as NOT RUN), runs **one benchmark at a time, base then head**, and
writes `summary.md` (one row per benchmark; a removed finding is marked `!!` at the top) and
`details/<bench>.md` (every added, removed, and changed finding, one per line: reachability, the
representative path, `other_sources`, the uncertainties by kind and by `depth_limit` reason, `stats`,
peak RSS; a run that did not converge is said so). Exit status: 0 nothing lost, 1 a finding was
removed, 2 something did not run. **`bench/compare.sh --help` prints the options and is the authoritative list** (`--wait`, `--timeout`,
`--gomemlimit`, `--resume`, `--only` / `--skip`, `--light` / `--heavy`, `--big`, `--fixed-inputs`, `--dry-run`, `--no-cache`);
`--dry-run` prints the plan. Before each run it waits until the free memory is 40% or more (macOS; elsewhere swap use is 50% or less)
and no other `rivus` / `rv-*` / `rivus-*` process runs. For a night:
`nohup bench/compare.sh --resume <base> <head> > compare.log 2>&1 &`. A big real codebase is run only when it is named (`--big NAME=URL@COMMIT`, the
periodic and occasional runs of `bench/explore/README.md`); the gate never runs one. The classification of step 5, and
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

**Rust: third-party code is read in the container only.** The extraction of a Rust target runs its `build.rs` and proc macros.
`compare.sh` therefore reads only our own code on the host (with `-unsafe-host-exec`): the paths listed in `OWN_RUST_PATHS` of the script
(`bench/zoo-rust`, `bench/rust-curated`, `extractor/rust/test`). Every other Rust target (parseable) is read by `rivus debug extract -lang rust`,
which is the default of rivus: the container, a fetch with the network and no build, then the extraction with no network
(docs/rust-isolation.md). The extractor that runs is the one of the version: it is built from `<work>/wt/<sha7>/extractor/rust` (once per tree), and the IR is cached per
that tree as before. Without the sandbox the third-party benchmark does not run (exit status 2, "Rust の第三者のコードは隔離の中でだけ読む。macOS のサンドボックス(sandbox-exec)が使えないので、このベンチを外す");
it never falls back to the host, and `RIVUS_UNSAFE_HOST_EXEC` is removed from the environment of that run. `--dry-run` shows `host` or `sandbox` per Rust benchmark;
`python3 bench/test_compare.py` pins the choice and the failure. `bench/parity/collect.sh` follows the same rule.

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
extractor or the fixtures changed too. A base that does not converge is the one `NOT CONVERGED` note, not an `INVALID`.

The synthetic tests of this report (exit statuses) are `python3 bench/test_compare.py`.

**Tiers (`--light`, `--heavy`).** The tiers are measured, not named. Light: one run of 60 seconds or less and 6 GB or less
(curated three, go-synth-S, `go-scale`, the zoos, gogs x 2, gin-vue-admin, go-test-bench, parseable, dart-pub, gophish, casdoor, and `gitea-cut`, the cut-out of the three CVEs of Gitea (7 packages, layer B; not the whole of Gitea; see "Gitea" below)). Heavy: the `--big` targets (none in the gate).
Within the light tier, base and head of a benchmark run at the same time when the free memory allows it (checked a few seconds after
base started: 35% free or more), and a light run waits only for free memory (25%), not for other analyses; a light-only run has its own
lock (`lock-light.d`), so it can run beside a heavy one of the same work dir. The time and the peak RSS of a light run are therefore
measured beside another run: read them as a guide. Heavy: one at a time, waits for a quiet machine, and never uses the cache; it is
for the periodic and the occasional runs, never the gate (`bench/explore/README.md`). The real codes (gogs etc.) stay in the light tier: they
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
6. Put the numbers and the classification in the commit message or the ADR (`docs/adr/`:
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
| ech0 | https://github.com/lin-snow/Ech0 (`bench/compare.sh` clones it) | `cecc2c19b590df85d79ea98457faa143130cd620` (the parent of the fix `091d26d2`) | built-in rules only in `compare.sh`; the measurements of ADR-047 used `examples/rules/go-gin.yaml` on the head of 2026-11-21 | not scored |
| parseable | <https://github.com/parseablehq/parseable> | `5c20b8e477be162238037e202cea1869d657017b` | `bench/parseable/rules.yaml` | `bench/parseable/truth-5c20b8e4.json` |
| dart-pub | <https://github.com/dart-lang/pub> | `5e24336e71a05a96b558d496953ebed223f6e40a` | `bench/dart-pub/rules.yaml` | `bench/dart-pub/truth-5e24336e.json` |
| gophish (GHSA-9c9w-9pq7-f35h, GHSA-hvw3-p9px-gpc9; `bench/wild/go/`) | <https://github.com/gophish/gophish> | `27d13a0584731371033926064d750e5f4f620e7a` | `bench/wild/go/GHSA-9c9w-9pq7-f35h/rules.yaml` | `bench/wild/go/gophish-27d13a05.truth.json` |
| casdoor (GHSA-9vm3-r8gq-cr6x; `bench/wild/go/`) | <https://github.com/casdoor/casdoor> | `7b0b426a76fd77b89817e0eafcccaed8d15b8cf4` | `bench/wild/go/GHSA-9vm3-r8gq-cr6x/rules.yaml` | `bench/wild/go/GHSA-9vm3-r8gq-cr6x/truth.json` |
| the cut-out of the three CVEs of Gitea (7 packages, `-no-deps`; **not used by the gate since 2026-11-21; the whole of Gitea is a target of the pass line C since 2026-11-24**, see "Gitea" below) | <https://github.com/go-gitea/gitea> | tag `v1.26.4` | `bench/gitea/rules.yaml` | `bench/gitea-cve-truth.json` |

How to run each is in the README of the target where there is one (`dart-pub/`, `parseable/`; the top of
this file for the Go ones). "Not recorded" is a gap to be filled the next time someone has the
checkout: write the commit here when it is known.

## The ledger of the heavy runs

`make bench-passline` and `make bench-deps` add **one row per run** to `bench/heavy-ledger.tsv`, whatever the result (the columns: the top of `bench/heavy_ledger.py`; notes: `bench/heavy-ledger.md`). Only the tools write it (one `O_APPEND` write under a lock on the file, so two runs at the same time keep their lines whole); a number in a PR or a document points to a row (date and commit) and is not copied. `NO_LEDGER=1` adds no row (a binary with a measurement patch, or a run that is not used as a number). The JSON of a result stays in `$BENCH_OUT` (default `.claude/work/bench-out/`, git-ignored; the ledger holds numbers only). A binary without `summary.memory_*` or `gc_cpu_fraction` gives empty cells, not an error.

```sh
python3 bench/heavy_ledger.py [--kind passline|deps] [--target memos] [--last N]   # read only; prints the margin (limit - peak, limit - live heap)
```

How to measure and what a line means: [`passline-baseline.md`](passline-baseline.md). The light benches: [`LEDGER.md`](LEDGER.md) (unchanged).

## The graph check (a change for memory or cost)

`bench/compare.sh --graph-check go-curated,zoo-go,go-synth-S,memos,dart-curated,zoo-dart <base> <head>` builds both versions, runs each on the same code, and compares what they
build: the IR (`rivus debug extract -all`; for Dart, the output of the extractor of that version, which `rivus index` and `rivus analyze` run too; `accept.Compare`, the comparison of the acceptance test, which compares two extractors; this one compares two
commits), the edges of the call graph with their candidates (`rivus index`), the summary of every function (the internal dump `RIVUS_INTERNAL_SUMMARY_DUMP`, written
by `Engine.WriteSummaryDump`: one line for each function with the hash of the whole summary, outputs and steps and sinks and guards; no flag, not in `-h`), and the
findings and uncertainties (as sets), and the same two arrays in the order of the output (`findings-order`, `uncertainties-order`: SHA-256 of the array with the order kept and only the keys of each object sorted; a change of the order is a change for the user). One line each: `same`, or `DIFFERENT` with how many and the first function IDs. The memory work shows "same" here instead of a count of
findings ([WORKFLOW.md](../docs/WORKFLOW.md)). The dump reads the fields of a summary, not how they are stored, so a change of the storage keeps it; a change of the
fields must change `internal/taint/summarydump.go` with it. The checks of the tool itself: `go test ./bench/graphcheck ./internal/taint -run SummaryDump`, `python3 -m unittest bench.test_graph_check`.
Dart (`dart-curated`, `zoo-dart`; [issue #211](../docs/issues/211-graph-check-dart.md)): each side runs the extractor of its own tree on the code of head (`RIVUS_DART` names the `dart` command), so the IR line also shows a change of the extractor. An analysis that stops (exit code 4: `zoo-dart` has cells that stop by the pass limit on purpose) is compared as far as it goes, and the line `convergence: NOT CONVERGED` makes the exit status 4 (same, and only the convergence is missing; a part not compared is 3, which wins over 4), never 0; the same line holds for a Go run that stopped by the timeout. `zoo-go` has such cells too. The weekly CI (`graph-check` in `.github/workflows/ci.yml`) runs each target by itself and accepts 4 for `zoo-go` only ([issue #217](../docs/issues/217-graph-check-zoo-go-not-converged.md)); `go-std10` and `go-curated` must end with 0, and 1, 2 and 3 fail every target; a clock gives only the `func_timeouts` line and never changes the status. Rust is not covered.
`go-std10` is the standard library: the 10 packages of `TestAcceptOrderStd` (`stdPatterns` of `internal/frontend/goir/accept_test.go`: keep the two lists the same), a source of shapes that the zoo and curated lack.
The names of `deps-targets.json` (memos, memos-4gib, gitea, dart_services) run under the heavy-work lock and the memory cap of the target; gitea is the pass line C (it passes now: see its newest `--kind passline` row of
`bench/heavy-ledger.tsv`; the numbers are there), so its graph can be compared.

## The cost warning (G12)

The cost table of `compare.sh` has four columns that do not depend on the run (`adds total`, `adds max of a function`, `passes`, `largest SCC`). A rise of 10% or more from base
to head is a line `warning: G12: ...` in `summary.md`, and in GitHub Actions a `::warning::` (never a stop). The job `cost-warn` of `ci.yml` runs it in a pull request that changes
`internal/taint`, `internal/sg`, `internal/ir`, `internal/callgraph`, `extractor/dart` or `bench`: `go-curated`, `dart-curated`, `go-synth-S` and `dart-synth-S`, `-M`, `-F`
(each of these runs takes a few seconds; F needs the Dart SDK). `dart-synth-*` is made by the generator of the tree of head, for both sides. WORKFLOW.md has the rule.

## One lock for the heavy runs

The tools that run an analysis of several GB (`make bench-passline`, `make bench-deps`, `bench/compare.sh --cost-check`,
`bench/go-scale/measure.py`, `bench/go-synth/measure.py`) take **one lock for the whole machine** first (`bench/heavylock.py`,
`fcntl.flock`: the OS releases it when the process ends). When another heavy run holds it, they print one line
(the PID and the command of the holder) and wait, at most `RIVUS_HEAVY_LOCK_WAIT_S` seconds (default 1800); after that they
fail (exit 75). The file is `$RIVUS_HEAVY_LOCK`, else `~/.cache/rivus/heavy-run.lock` (a fixed place per user, the same for every work tree).
A tool started by a tool that holds the lock does not wait for itself (`RIVUS_HEAVY_LOCK_HELD=1`). There is no way to force the
lock off: to run now, end the process that holds it. For a shell tool: `python3 bench/heavylock.py -- <command...>`.

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
| `go-scale/` | a generator of Go code of about 2,000 functions, each profile with ONE shape that made Gitea and gogs big (a cache of types, `os.Stderr`, widen, a ring of 200 functions, ...), with its answer key in the source (the marks of `go-curated`), and `measure.py` | does the engine still stay small on the shapes of scale, and keep the real flows next to them? Gitea is the last check, not the first | 1 to 7 seconds a profile, in `go test ./cmd/rivus -run TestScaleSynth` (skipped by `-short`) |
| `parity/` | the table of what the built-in model gives each language (B-52, stage 1): the quantity of the model, the demand of real code (the calls to functions without a body that have a row), what is checked by a run, and the rows that no IR hits (dead rows), side by side with the counts, by shape ((i) returns a value, (ii) writes to an argument, (iii) calls a callback, (iv) rules); `TABLE.md` is not committed (made only when it is needed; `README.md` says how). Where the parity of the languages stands is in the marks of the zoo (`@na`, `@todo(PARITY)`) and the gate of `bench/zoo` | which language is behind, and where: a row that is spelled in a way the extractor never writes, a shape the default gets wrong | a few minutes (the analyses of the real code), the table in 10 s |
| `dart-curated/`, `rust-curated/` also have `rust-curated-ffi/` and `rust-curated-lib/` | two small crates (a binary with a `#[no_mangle]` function, a library crate) for the case where a program is called from outside, so what is `pub` is not a closed world (`docs/DESIGN-capture.md`) | the same | seconds |
| `zoo-go/`, `zoo-dart/`, `zoo-rust/` (and `zoo/`) | cells of one shape each, with answers that a run backs (`zoo/spec.json`, [`zoo/README.md`](zoo/README.md); design in `docs/DESIGN-zoo.md`): `TestZooGo`, `TestZooDart`, `TestZooRust`, `TestZooPureGo` | does each shape of the model and the engine give the answer that is written for it, per language? | seconds to a minute, in `go test ./...` (Dart and Rust with `RIVUS_DART` / `RIVUS_RUST`) |
| `wild/` | published vulnerabilities of real code (the SHA and a fetch script only; the code of the target is not copied; [`wild/README.md`](wild/README.md)): gophish, casdoor (and the Gitea three, by `bench/gitea`) | does the engine find the real flow of a published advisory? | per target |
| `gcp/` | a Go module that registers handlers of Cloud Spanner and BigQuery (nothing connects to Google Cloud); the case for the size of the summaries from the dependencies (`explore/LOG.md`, measurement 3, with `rules-stored.yaml`) | where do the elements and the time of the dependencies come from? | a measurement, not a gate |
| `explore/` | the periodic and occasional runs of big real codebases, out of the gate: `README.md` (the procedure), `pool.yaml` (the targets), `LOG.md` (the log; its first sections are about the cut-out of Gitea, kept as records) | what does the engine do on a code it has not seen? | minutes to hours |
| `ech0/` | **no files but a README, and no truth**: Ech0, GHSA-8mc6-xjpr-h98x (SSRF), commit `cecc2c19`. The flow is **second-order** (an administrator adds a peer URL, it is stored in the database, and `fetchPeerConnectInfo` requests it later); rivus has no model of what a database holds, so the flow is not found, and `SendRequest` is a project function that needs a project rule for the sink. Recorded as a limit of what rivus can see, not as a miss in the scoring (`bench/ech0/README.md` has the same text) | not scored | |
| `gogs/`, `gin-vue-admin/`, `go-test-bench/`, `dart-pub/`, `parseable/`, `dart-corpus/` | the truth files and rules of the targets above (a directory has what the table says; the targets themselves are not here); `dart-corpus` is the regression of the Dart extractor on 54 packages | see the table of targets | |
| `LEDGER.md`, `BACKTEST.md`, `parity/` | the ledger of versions, the backtest of the light benches against the regressions that only the heavy projects showed, and the tool of the parity tables | see each file | |
| `dart-synth/`, `rust-synth/` | generators of a Dart package / a Rust crate (depth, SCCs, closures given to shared helpers, relays, closures in fields, captures; size F of Dart: closures of the project given to a dependency package, which gives them on to other functions of it); the plan is `synthplan/`; `go-synth/measure.py -lang dart` or `-lang rust` runs the extractor once and measures `rivus analyze -ir` | the same | S under a second, M and L seconds |
| `model-author/` | a tool (Go; `make model PKG=<…> VERSION=<…>`): the steps of making the model of a Go package that a script can do (stage 1 of `docs/DESIGN-package-models.md` 6.11). It fetches the source (fetch only, nothing of it is run), lists the candidates of layer 1 by a table of rules about signatures (`rules.go`), matches each with a row or an entry of `rejected:` in the model file, and stops with exit code 70 when one is neither; `-recall` reports how many adopted sink and source rows the table finds; `-diff` runs `model-diff` and writes the material for the pull request; `-write-verified`, `-write-dead-allow` write the mechanical parts of the file | exit codes (0, 2 usage or a file the loader refuses, 10 fetch, 20 reading the package or the differential test, 50 the differential test failed, 70 undecided candidates; 30, 40, 60, 80 are kept for later stages), the list of undecided candidates, the count of layer 2 | seconds per package. By hand (not in CI) |
| `model-diff/` | a tool (Go; `make model-diff PKG=<…>`): the model of one package against its body. It calls each function of the model's rows with a source in each position, one sink per function and place, and runs the same code with the bodies read (the rows of the package left out) and with the models; a `sinks` row is checked against its declaration (the source in one position at a time, and a control with constants); a `pure` row must not lose a flow that the body has (to the result, to a callback, from the receiver); a sanitizer row is read against the body (the value has to reach the result, and the flows of the row have to be the body's); a source row is looked up in the package (the symbol, the field, the type and the positions). A row whose symbol is not in the package fails. Go and Dart (Dart: flows and sinks only); Rust is not checked (the build of a crate needs the isolation) | MISS (the model lost a flow), EXTRA (the model is wider than the body; allowed by a `# model-diff: extra` note in the model file with a reason; a MISS that was checked against the body and is not a flow of data is recorded by a `# model-diff: miss-reviewed` note of one of three kinds, `type-only`, `non-payload`, `body-default`: the table is in `docs/MODEL-RUNBOOK.md` section 9), and what could not be checked (generics, callbacks of a row that is not pure, types it cannot build: listed as unverified; every row is counted as judged or not judged, per kind) | seconds per package, about 1 GB with a Dart package. In a pull request that changes a model file: the packages of the changed rows only (`make model-diff-changed BASE=<sha>`, Go). Weekly and by hand: a list of packages. How to use it: `docs/MODEL-RUNBOOK.md` section 9 |

When the core of the engine (`internal/sg`, `internal/sites`, `internal/taint`) is touched: run
`go test ./...` (this includes `go-curated`; with `RIVUS_DART` and `RIVUS_RUST` set, `dart-curated` and
`rust-curated` too), then the S and M of `go-synth`, `dart-synth` and `rust-synth` before and after with the
same binary built from each state, and check that the result is identical (or that every changed
finding is understood). The gate is the light tier of `bench/compare.sh` (about 10 minutes); a big real
codebase is for the periodic and occasional runs of `bench/explore/README.md`, never a step of a change. See `go-synth/README.md` for how, and `docs/adr/046-own-bench-curated-synth.md` (ADR-046) for why.

Two more things are held by the three curated tests. Each `@miss` says whether the engine left a
trace of the flow it lost: `@miss(silent)` (nothing in the functions of the source and the sink; 51 of the
78: Dart 31, Rust 15, Go 5) or `@miss(<kind>)` (27: Dart 16, Rust 1, Go 10); and the findings whose path has no step in
the function of their source are counted against a ceiling (BENCH-2; 0 in the three, 0 of 76 in
`TestDogfood` (2026-11-22; it was 39 earlier) and `paramshare` too: CORE-B was fixed, ADR-049). See `MISSES.md`.

```sh
export RIVUS_DART="mise exec dart@3.13.6 -- dart" RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust
go test ./cmd/rivus -run Curated -v        # the three accuracy benchmarks: counts in the log
```

## The share of silent misses

(The classification of each `@miss` by cause is in [`MISSES.md`](MISSES.md); this section is the tally and stays here.)

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

**Gitea (a note of 2026-11-22; changed 2026-11-24).** The user decided on 2026-11-24 that the whole of Gitea (`v1.26.4`) is the target of the pass line C (`passline-baseline.md`): it may be fetched and run for that. Periodic runs and exploration on it stay a separate decision. Before that, from 2026-11-21: it is not in the gate and not a step of a change; the exploration
uses small reproductions and rotating targets (`explore/README.md`). The files below are **still in the repository** and are not removed: the
cut-out `gitea-cut` is still in the light tier of `compare.sh` (`--light` and the default list), and `bench/gitea/` (`rules.yaml`, `truth-v1.26.4.json`) and
`bench/gitea-cve-truth.json` are still here. Whether the cut-out leaves `compare.sh` is not decided in these documents. The rest of this section
describes them as they are.

**`bench/gitea/truth-v1.26.4.json`** is the first form of the ground truth of those three CVEs ((file, line, category) of the sink, for a run of the whole of Gitea). No tool grades it now: `bench/score.py` and `compare.sh` use `bench/gitea-cve-truth.json`. It is kept because `bench/gitea-cve-paths.md`, `bench/wild/README.md`, `bench/go-curated/README.md` and ADR-055 refer to it as the definition of the labels; do not delete it without changing them.

**The cut-out of the three CVEs: use the tag.** The truth file `bench/gitea-cve-truth.json` is for the tag `v1.26.4` (the three CVEs are in that
code, and fixed in later releases). `bench/compare.sh` (`gitea-cut`, 7 packages, `-no-deps`) checks out the tag in its own copy. A copy of the
repository that was cloned some other time is the head of `main` of that day, not the tag: a measurement on it (the time, the
passes, the recall of the three CVEs) is of other code, and has to be taken again on the tag. Check with
`git describe --tags` (or `git rev-parse HEAD` against the tag) before a measurement is written down.

## The scale profiles (`go-scale/`)

**What it is for.** The phenomena of scale appeared only on Gitea and gogs (6 to 16 minutes, 5 to 13 GB). Each of them has a cause that
is known (ADR-049, ADR-055, ADR-056, ADR-057), and each can be built at 2,000 functions in 1 to 7 seconds. `go-scale/gen` writes the module
(`go run ./bench/go-scale/gen -shapes types -out DIR`; `-shapes` takes one or more of the names below, `all` or `none`; the same flags write the
same bytes; the output is not committed). Every profile is the same ballast (2,000 functions that call each other, 4% of them with a source, no
sink) plus ONE shape, so that a change of a number has one cause. The code carries its answer key in the marks of `go-curated`
(`@source(tag)`, `@sink(tag)`, `@sink(none)`, `@nosink(tag)`): the flows that must stay (`*_real`: a real flow next to the confluence) and the sinks
that must stay quiet. A change that cuts too much (it loses a `*_real` flow) fails as surely as one that cuts too little.

| profile (`-shapes`) | the phenomenon | the change that fixes it | what it does without that change (the engine of `2dd5022`) |
| --- | --- | --- | --- |
| `types` | the data of a value reaches a cache of types (`reflect.TypeOf` / `(reflect.Value).Type` as `proto.marshalInfoMap`), and every reader of the cache | the `reflect` models (ADR-055) | `summary_elems` x11, largest summary 6 to 76, `reanalyses` x1.8, 2 passes (not 1), 72 false pairs into the `@sink(none)` |
| `stderr` | `fmt.Fprintf(os.Stderr, tainted)`, and every `log.New(os.Stderr, ...)` carries it | `holds: nothing` (ADR-056) | `summary_elems` x13, largest summary 6 to 85, `reanalyses` x1.9, 3 passes (not 2), 82 false pairs |
| `widen` | an output of more than 24 elements is widened to the whole parameter; the join of the whole and the parts | `absorbWhole` (ADR-057) | **the parts are kept next to the whole**: largest summary 7 to 47, `summary_elems` +3%. **The going back and forth (63, 82, 85, 63) and the 12 passes of gogs are NOT reproduced** (what is needed is a widened output and an unwidened one of the same function that alternate between passes; the random rings tried converge at 2 passes with and without the change) |
| `scc` | a ring of 200 functions (and 40 writes of sources into a global array that nobody reads), R-43: `-workers 1` gives other numbers than 2 or more (issue #96) | (none: a shape of cost) | the same on both: the shape is for `reanalyses`, `passes`, and for R-43 (`scc-w1.json` and `scc-w4.json` differ: `summary_elems` 13,859 and 11,487, `reanalyses` 3,846 and 4,188) |
| `typeschain` | the cache of types registers what it knows in a second cache, and that in a third, 14 stages, each a package variable: the type of a value goes one cache further in a pass | the `reflect` models (ADR-055) | **does not converge**: 12 passes (`MaxPasses`), `converged: false`, largest summary 144, and nothing of the false pairs yet (they would come after 14 passes). With 8 stages: 9 passes (stages + 1), `summary_elems` 57,727, 72 false pairs. The hypothesis "a type descriptor goes through one global a pass, so the types confluence times a chain does not converge" is **confirmed on this shape** (not shown for Gitea itself) |
| `chain` | a chain of globals, each written from the one before (one step a pass). The number of passes follows the number of steps: 3 steps 4 passes, 10 steps 11 (default), 13 and 30 steps **12 passes, not converged, the flow is lost** (`@miss(silent)`; one `recursion_approximation`, in neither the source nor the sink function). `chain3`, `chain`, `chain13`, `chain30` have their expected files | | the same on both |
| `dyn` | an interface with 150 implementations, 40 callers under conditions; a call on one implementation must not see the others | | the same on both (the multiplication of ADR-047 is not reproduced: no closures) |
| `glob` | a handler writes a global, another entry reads it (the explicit write that must stay: CORE-H) | | the same on both, 2 flows found |
| `giant` | one function of 3,000 statements (it takes 5 s; it is stopped by work, never by a clock) | | the same on both |
| `selfcopy` | a struct copied out of a global and written back, 40 times (the shape of C-7's gogs +45): what is in the pointee must not reach the other field | | the same on both: **the amplification of C-7 is not reproduced** (1 pass, 0 false) |
| `middleware` | the shape of the AWS SDK (smithy-go, the largest summary of memos): a stack of 3 steps, each a linked list of stages that its `HandleMiddleware` chains with the handler after it (`s.tail.Next = next`), 9 stages shared by 40 operations, each putting them into a stack of its own with functions it calls on the stack (`APIOptions`) and calling it through `Handler.Handle` / `Middleware.HandleMiddleware` (one interface, every stage and every last handler a candidate); the request is a struct of 32 fields that the stages write; 1 in 8 functions of the ballast calls an operation | (none: a measurement) | the same before and after the models of the interfaces are joined with the bodies (ADR-075; the chain is the project's own interface, so nothing changes): **the swelling is NOT reproduced**: `summary_elems` 5,319 (the ballast alone: 5,085), largest summary 6, 1 pass, `reanalyses` 2,194 (ballast 2,026), 2 flows found, 0 false. Growing it (200 operations x 30 stages, 400 x 60) grows the numbers in a line, not in a power (`summary_elems` 6,182 and 7,272, 1 pass, largest summary 6). What makes memos 985,000 elements in 3 passes is not in this shape: the summary of the chain is small, and the chain's cost in memos is in the dependency itself (`dep_spent_ms` 3.78 M of 3.80 M). A candidate to try next: the stages as function values (`HandlerFunc`), and the options as closures that capture the operation's own values, which is the multiplication of the function values (the case for narrowing the candidates, "plan A": the candidates a call can have, by what the caller handed over) |

Not made yet: the edge of `valuecalls` (8 and 9 callers) and the multiplication of ADR-047 (208 candidates with a guard each), which need
closures given to a helper; Dart and Rust profiles (a task of their own).

**What `TestScaleSynth` checks** (`cmd/rivus/bench_scale_test.go`, run in `go test ./...`, skipped by `-short`; `-workers` is fixed, 1, and for `scc` 1 and 4):
1. the answer key (the same function as `TestCuratedGo`): every flow that must stay is found, no sink that must stay quiet is reached, no other flow;
2. `bench/go-scale/expected/<profile>-w<workers>.json`: the set of the findings, the uncertainties by kind and `converged` are **the same**; `passes`,
   `reanalyses`, `summary_elems` and the largest summary must not be **more than 10% over** the file (a different number inside 10% is logged, a
   smaller one is an improvement to write down). The analysis is deterministic, so a number does not move without a change of the engine. Time is not
   looked at (it moves with the load). A change that is meant to move them: `RIVUS_SCALE_UPDATE=1 go test ./cmd/rivus -run TestScaleSynth`, and the diff
   of the files is in the commit. When R-43 (#96) is fixed, `scc-w1.json` and `scc-w4.json` become one.

**Measuring by hand and between two builds.** `python3 bench/go-scale/measure.py -rivus BIN -shapes types [-workers 1]` prints one line (functions,
passes, converged, reanalyses, `summary_elems`, the largest summary, found / missing / false against the key). `bench/compare.sh` has `go-scale`
in the light tier (all shapes in one module, about 2,550 functions, 11 s; the same input for base and head, made by the generator of the tree the script is in).
Against `2dd5022` it says "164 findings REMOVED" (the false pairs of `types` and `stderr`: that is the point of the profile; classify them by the key).

**Which change makes which profile big (measured once, 2026-11-21; the head with ONE of the three taken out, the others kept).**
Taking the fix out is not kept as a switch in the engine (no flag for it); it was done on a copy of the tree: `go-pure.yaml` without the five `pure: true`
of `reflect`; `holds: nothing` changed to `holds: everything`; `widen(absorbWhole(list))` changed to `widen(list)` in `joinSummaries`.

| the profile | head | without the `reflect` models | without `holds: nothing` | without `absorbWhole` |
| --- | --- | --- | --- | --- |
| `types` | elems 5,085, maxsum 6, 1 pass, 0 false | **56,869**, 76, 2 passes, 72 false | 5,085 | 5,085 |
| `stderr` | 5,006, 6, 2 passes, 0 false | 5,006 | **65,815**, 85, 3 passes, 82 false | 5,006 |
| `widen` | 5,198, maxsum 7 | 5,198 | 5,198 | 5,390, **maxsum 47** |
| the others | | the same as head | the same as head | the same as head |

Each fix moves only its own profile. `2dd5022` itself: `types` and `stderr` as in the columns above, `widen` 5,390 / 47, and the rest as head.
The three fixes together on all the shapes at once (2,547 functions, `-shapes all`, 2 s on head): `summary_elems` 78,190 to 14,141 (5.5 times), `reanalyses` 7,656 to 4,302, largest summary 164 to 45, 164 false pairs to 0, **base does not converge (12 passes), head converges in 11**, 52 real flows found by both.

**How big the profiles are (head, `-workers 1`, an idle machine):** types 2,033 functions 2.9 s; stderr 2,049, 2.1 s; widen 2,045, 1.1 s; scc 2,229 (largest SCC 200), 2.4 s;
chain 2,038, 0.5 s (11 passes); dyn 2,222, 0.5 s; glob 2,030, 0.5 s; giant 2,028, 2.2 s; selfcopy 2,067, 1.3 s; all together 2,529, 11 s. The shapes show their
effect at this size (the ratio is 11 to 13 for the confluences); the real cost of Gitea (60,000 functions, 8 passes) is not reached, and is not the point: the profile
says the shape is there; how large it grows on a real codebase is what the occasional exploration (`bench/explore/README.md`) is for.
