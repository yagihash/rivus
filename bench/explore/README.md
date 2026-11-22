# The occasional exploration

The gate of the bench (`bench/compare.sh`, about 10 minutes) is the light tier (`bench/BACKTEST.md` says why it is enough). No big real
codebase is a step of a change, and none is a fixed target: what the big ones give, new phenomena of scale, is kept by runs that are made
**outside the gate, now and then, on targets that change** (a target that is always the same finds nothing new after the first time). Nothing here is a gate; a run that goes wrong is a measurement,
not a failure.

**The rule of the user (2026-11-22): ask first.** A run on a real external project takes long and slows the development, so before each such need,
ask the user (through the advisor): what it is for, which target, the time limit. Nothing is run on a target of `pool.yaml`, or on a new one, without it;
only what the user has allowed is added to the list. Fetching and reading a target is fine; the run is what needs the answer.

## One run

A run has **two targets: one new, one from the last run** (the second says whether a number moved by the engine or by the target).

1. **Choose.** In `pool.yaml`, the new target is the one with `last_run: never` (the first of the list that nobody has marked) or, when
   all have run, the one run the longest ago. The target of the last run is the previous section of `LOG.md`. Write the commit in
   `pool.yaml` first (a full SHA; `~` is fixed now with `git ls-remote <url> HEAD`).
2. **Make room.** Only when the machine is quiet: no other `rivus` / `rv-*` process, free memory 40 % or more (`memory_pressure`), nothing
   else heavy running. A run on a loaded machine is not a measurement (`LOG.md`, Anomaly 1: 865 s against 198 s).
3. **Copy.** `git clone`, `git checkout <commit>`, `go list ./...` (the module must load). The copy lives in the work directory of
   `compare.sh`, never in this repository (`bench/wild/README.md`: the code of a target is not copied here).
   **A Dart target** (`lang: dart`): `git clone`, `git checkout <commit>`, and `dart pub get` in the package, and nothing else of the target: never
   `dart run`, `dart test`, `dart build` or `build_runner` (the code of the target is not run; a hook such as `hook/build.dart` runs only at a build).
   A workspace that has a Flutter package cannot be resolved without the Flutter SDK: take that member out of the `workspace:` list of the root
   `pubspec.yaml` of the copy (a change of the copy, never committed). Run it with `RIVUS_DART_EXTRACTOR="dart run <rivus>/extractor/dart/bin/rivus_dart.dart"`
   and `rivus analyze -lang dart -C <package> -timeout <limit>`; a part that cannot be read for the code that is not generated is said in the section.
4. **Run** the head build, `-timeout 5400s`, `GOMEMLIMIT` at 70 % of the free memory, `-pass-stats` when the build has it, rules: the built-in ones
   (a framework rule file only when the target uses it, named in the section):
   `rivus analyze -timeout 5400s -pass-stats ./... > out.json` and `/usr/bin/time -l` for the peak memory.
5. **Read** `stats` (functions, passes, `converged`, `reanalyses`, `summary_elems`, `largest_summary_elems` and the function that has it,
   `dep_spent_ms` against `own_spent_ms`, `levels`, `largest_scc`), the uncertainties by kind, the findings by category, the wall time.
   The numbers that look wrong are the point: a `summary_elems` of ten times the functions, a pass count near `MaxPasses`, a function
   whose summary is in the hundreds, time that is mostly `sys`.
6. **Write a section in `LOG.md`** (the format below), and update the row of the target in `pool.yaml` (`size`, `last_run`).
7. **A shape of scale is not left in the log.** When a number is unexplained and the cause is found, reduce it to a go-scale profile
   (`bench/go-scale/gen`, a new shape with an expected file) or to a corpus of the layer C, so that the gate sees it next time; the section says which.
   When the cause is not found, say what was tried; the target stays in the pool as "not explained".

## The format of a section of LOG.md

```
## Exploration N: <target A> (new) and <target B> (again) (<date>)

**Binary and inputs.** engine commit, target commit, rules, flags, machine state (free memory before, load).

| target | wall | user CPU | peak memory | functions | passes | converged | reanalyses | summary_elems | largest summary | findings | uncertainties by kind |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A | | | | | | | | | | | |
| B (again) | | | | | | | | | | | |

**What moved** (B against its last section, same engine or not: say which) and **what looks wrong** (A: a number, why it looks wrong).
**Reduced to**: a go-scale profile / a corpus of the layer C / nothing yet (what was tried).
```

Findings of a third-party target are counted, never described by place (the repository is public; a flaw with no public ID is not written with its place).

## The periodic run (outside the gate, never makes a push wait)

Besides the occasional exploration above, a **big codebase of the pool is run on a schedule that has nothing to do with a push**: once for
about every 10 commits that touch the core (`internal/taint`, `internal/sg`, `internal/model`, the built-in YAML), or once at the end of
the day, whichever comes first. What touches the core is not decided by "touches the convergence": a flaw of the models and of the dynamic
calls is not a flaw of the convergence.

1. Take the target of the schedule from `pool.yaml` (the one run the longest ago; the same rule as above). The runs are made on a quiet
   machine, in the background; nobody waits for them.
2. `bench/compare.sh --heavy --big NAME=URL@COMMIT <the commit of the last periodic run> <head>` (a build of the last periodic run against the
   head, on the same target). The commit of the last periodic run and its numbers are in the table below.
3. A difference (a removed finding, an addition beyond the NOISE line of `compare`, a run that does not converge, a size over 10 percent)
   is: (i) a bisection of the commits between the two runs, to the commit that did it; (ii) a small bench of the same shape (a zoo cell or a
   go-scale profile) so that the gate sees it next time; (iii) a row in `bench/LEDGER.md`.
4. Write the run in the table.

Last periodic run (the engine commit, the target, the counts; counts only, no place of a third-party finding):

| date | engine | target and commit | functions | passes | converged | summary_elems | findings | wall | peak memory | difference from the run before |
|---|---|---|---|---|---|---|---|---|---|---|
| (none yet) | | | | | | | | | | |
