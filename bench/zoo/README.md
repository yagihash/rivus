# zoo: cases of one shape each, with answers that a run backs

The design is in [`docs/DESIGN-zoo.md`](../../docs/DESIGN-zoo.md). This is how to work with what is built
(stage 1: Go, `bench/zoo-go`; the Dart and Rust zoos follow the same `spec.json`).

**The answer is what the case does when it runs, not what rivus says.** A source returns a marker; a sink
records what it got and does nothing else. The three runs of the cases (`TestZooExecGo`) say whether the
dangerous part of the marker reached each sink, and `TestZooTruthGo` holds the annotations to that. When
rivus disagrees with an annotation, the annotation stays and the case gets `@miss` or `@fp` (a ratchet: the test
fails the day rivus agrees, and the mark is to be removed). **Never change an annotation to what rivus says.**

## Files

| file | what |
| --- | --- |
| `bench/zoo/spec.json` | the marker, the dangers of both variants, the judgement of "reached" of each class, the probes. Every language copies it into constants and checks them against this file |
| `bench/zoo-go/mark` | what the cases call: `Src`, `SrcBytes`, `NewIn` (a parameter-type source), `SinkCmd`/`SinkCmdBytes`/`SinkSQL`/`SinkHTML`/`SinkPath(base, s)`/`SinkAny` (structs and containers only), `CleanCmd` ..., `Case`. `mark.go` is what rivus reads: declarations with empty bodies. `mark_exec.go` (`-tags zooexec`) records |
| `bench/zoo-go/rules.yaml` | the meaning of `mark` for rivus: public rules only |
| `bench/zoo-go/cells/<family>/<shape>/` | one cell = one package = three cases |
| `bench/zoo-go/cmd/observe` | the harness (`-tags zooexec -cover`); `imports.go` lists every cell (the test checks it) |
| `bench/zoo-go/pure`, `cmd/pure` | calls the `pure` rows of the built-in model with the marker (R-03) |
| `cmd/rivus/zoo_*_test.go` | `TestZooExecGo`, `TestZooTruthGo`, `TestZooGo`, `TestZooMarkIndependent`, `TestZooPureGo` |

## Adding a cell

1. `cells/<family>/<shape>/<shape>.go`, package `<shape>` (underscores), with in the package comment
   `@cell(F/<family>/<shape>)` and `@shape: <the shape in one line>`. Add the package to `cmd/observe/imports.go`.
2. Three cases, registered in `init` with `mark.Case("F/<family>/<shape>/vuln" | "fixed" | "decoy", fn)`; the function
   has no argument and no result, and the line of its `func` has `@vuln`, `@fixed` or `@decoy`. A cell with more than one
   vulnerable case (the chains of 3, 13 and 30 stages) names the others `vuln13` ...
3. Annotate (the same syntax as the curated projects, plus the ones below). Tags are `[a-z0-9-]+`, one source per tag,
   and the tag of the source is the tag given to `mark.Src` / `mark.NewIn`:

   ```go
   v := mark.Src("call-direct-vuln") // @source(call-direct-vuln)
   mark.SinkCmd(v)                   // @sink(call-direct-vuln)
   ```

   * **vuln**: the value reaches the sink: `@sink(tag)`.
   * **fixed**: a sanitizer first; the sink line is `@sink(none)`. (No `@source` there: a tag with a source needs a sink.)
   * **decoy**: the same shape, but the tainted value goes to another object, argument or field and reaches a *different* sink
     line (`@sink(tag)`, the flow that lives beside), and the line that must stay quiet is `@sink(none) @nosink(tag)`. The
     decoy must run that quiet line (a `@sink(none)` that never ran proves nothing).
   * Every statement of the cell must run (a branch: call the case twice with the condition changed).
   * A sink of another class (`SinkSQL`) beside `SinkCmd` is the usual way to give the tainted value a different line.
     The sanitizer of a class does not clean the others, so a `SinkAny` cell (all classes) cleans with all four.
4. `go test ./cmd/rivus -run 'TestZoo' -v`. A line that the run contradicts, a missing role, a statement that did not run,
   or a reverse run that saw something else fails `TestZooExecGo` / `TestZooTruthGo`. Then see what rivus says.

### When rivus does not agree

* A flow that rivus does not find: `@source(tag) @miss(silent) @ref(<item>)` (the flow is there; `silent`: no uncertainty in
  the function of the source or of the sink; or `@miss(<kind>)`, the kinds of the curated projects; the test says which one
  is true). `@miss(recursion_approximation)` is for a run that stopped by its pass limit: the uncertainty is of the whole
  program, and `converged: false` is then expected (and checked from both sides).
* A report that must not be there: `@sink(none) @fp @nosink(tag) @fp @ref(<item>)` (the `@fp` goes after `@sink(none)` and after
  `@nosink(...)`).
* `@ref(...)` is required on both (a review item such as `CORE-J`, an issue, a CWE). A `@miss` without it fails.
* `@manual(<reason>)` on a sink line: only for what a run cannot show. At most 3% of the cases and one to a cell.
* When the whole zoo fails, each cell is analysed alone (`ZOO_ALONE=1` asks for it): green alone and red in the whole is a
  mix with the others (the engine's problem); red alone is the cell.

### Fixing the engine

A change that makes a `@miss` true no longer fails the test with "found now (remove @miss)": remove the mark in the same
commit. Do not change the case to make it pass: the commit that changes a cell and removes a `@miss` or `@fp` together is the
one to look at (`@shape` says what the cell is about; if the shape changed, say why).

## The headline numbers

`go test ./cmd/rivus -run 'TestZoo' -v` prints, per language, one line:

```
zoo-go: cells 24 / cases 74 | found 20 / 26 | @miss 6 (trace 2, silent 4, no rule 0) | fixed reported wrongly 0 | decoys reported wrongly 2 / 24 | carry map[] | manual 0 (0.0%) | run and annotation disagree 0 | statements not run 0
```

* **cells / cases**: cells with all three roles, and their cases.
* **found a / b**: of the vulnerable tags, those that rivus reports (the others are `@miss`).
* **@miss**: the vulnerable tags that are not found, split by what the engine left: a trace (a kind), `silent`, `no rule`
  (`@miss(norule)`: no rule exists for the API; stage 3).
* **fixed / decoys reported wrongly**: `@fp` on the quiet lines of the fixed and decoy cases.
* **carry**: `@carry(tag, encode|digest|numeric)` lines by kind (a value that depends on the marker and has not reached:
  the text differs between the two variants).
* **manual**: `@manual` lines. **run and annotation disagree**: always 0. **statements not run**: always 0.

`TestZooPureGo` adds one more line: the `pure` rows of the model that were called with the marker, and how many depend on it
(R-03; `knownWrongPure` holds them as a ratchet).
`TestZooMarkIndependent` holds that the findings are the same with the real implementation of `mark` and the registration
of the cases in the analysis (the design says the analysis must not see them).

## Light tier

`bench/compare.sh --light` has `zoo-go` (`./...` of `bench/zoo-go`: the cells and `mark`; the harness, the `pure` package and
`cmd/` are behind the `zooexec` tag). A cell added in head shows as `+N` of that benchmark.
