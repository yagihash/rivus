# zoo: cases of one shape each, with answers that a run backs

The design is in [`docs/DESIGN-zoo.md`](../../docs/DESIGN-zoo.md). This is how to work with what is built
(stage 1: Go, `bench/zoo-go`; stage 2: Dart, `bench/zoo-dart`, see the last section; the Rust zoo follows the same `spec.json`).

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
zoo-go: cells 29 / cases 89 | found 25 / 31 | @miss 6 (trace 2, silent 4, no rule 0) | fixed reported wrongly 0 | decoys reported wrongly 2 / 29 (with the trace container_element_imprecision 2, without 0) | carry map[] | manual 0 (0.0%) | run and annotation disagree 0 | statements not run 0
```

* **cells / cases**: cells with all three roles, and their cases.
* **found a / b**: of the vulnerable tags, those that rivus reports (the others are `@miss`).
* **@miss**: the vulnerable tags that are not found, split by what the engine left: a trace (a kind), `silent`, or
  `no rule` (`@miss(norule)`: the API has no rule. Only for the APIs of the standard library and of the de facto
  libraries; an API outside that scope is not counted as a miss. Stage 3; none in stage 1).
* **fixed / decoys reported wrongly**: `@fp` on the quiet lines of the fixed and decoy cases. The decoy count is split: a finding that carries `container_element_imprecision` is an over-approximation that says so; one without it is a false report with no trace.
* **carry**: `@carry(tag, encode|digest|numeric)` lines by kind (a value that depends on the marker and has not reached:
  the text differs between the two variants).
* **manual**: `@manual` lines. **run and annotation disagree**: always 0. **statements not run**: always 0.

`TestZooPureGo` adds one more line: the `pure` rows of the model that were called with the marker, and how many depend on it
(R-03; `knownWrongPure` holds them as a ratchet).
`TestZooMarkIndependent` holds that the findings are the same with the real implementation of `mark` and the registration
of the cases in the analysis (the design says the analysis must not see them).

## Light tier

`bench/compare.sh --light` has `zoo-go` (`./...` of `bench/zoo-go`: the cells and `mark`; the harness, the `pure` package and
`cmd/` are behind the `zooexec` tag). A cell added in head shows as `+N` of that benchmark. `zoo-dart` is the same for Dart (`bench/zoo-dart/cells`; the Dart extractor, about 9 s and 1.1 GB).

## Dart (`bench/zoo-dart`)

The same marker, the same judgement (`spec.json`), the same annotations and the same checks as Go. What differs:

| file | what |
| --- | --- |
| `bench/zoo-dart/cells` | the package that rivus analyses (`rivus analyze` reads this directory; `compare.sh` has `zoo-dart`). `lib/<family>/<shape>.dart` is one cell = one file = three cases; `rules.yaml`; `packages/zoo_mark` has the **declarations** of the marks with empty bodies (`src`, `In`, `sinkCmd`/`sinkCmdBytes`/`sinkSql`/`sinkHtml`/`sinkPath(base, s)`/`sinkAny`, `cleanCmd` ..., `zooCase`) |
| `bench/zoo-dart/exec` | the harness: the recording `zoo_mark` (`exec/zoo_mark`, put in place of the declarations by `dependency_overrides`), `lib/cases.dart` (the list of the cells' `register()`; the test checks that every file of `cells/lib` is there), `test/zoo_test.dart` (one `test()` per case) and `test/spec_test.dart` (the constants and the probes of `spec.json`, in both variants). rivus does not read this directory |
| `cmd/rivus/zoo_dart_test.go` | `TestZooExecDart` (`dart test --coverage` three times: variant 0, variant 1, variant 0 in the reverse order, in different processes; the lines of `cells/lib` that no run executed fail), `TestZooTruthDart`, `TestZooDart`, `TestZooMarkIndependentDart`. They share the comparison of the runs, the checks of the cells and of the lines, and the headline with the Go ones (`zoo_test.go`). `RIVUS_DART` names the Dart command; without it they skip (`RIVUS_REQUIRE_DART=1`: they fail) |

Adding a cell: `cells/lib/<family>/<shape>.dart` with `// @cell(F/<family>/<shape>)` and `// @shape: ...` in the first lines, `void register() { zooCase('F/<family>/<shape>/vuln', vuln); ... }`, the three functions with `// @vuln`, `// @fixed`, `// @decoy` on the line of their declaration (more vulnerable cases are `vuln2` ... with their own tags: `<family>-<shape>-vuln2`), and the file in `exec/lib/cases.dart`.
The rest is as for Go (steps 3 and 4 above). Things that are Dart's:

* The position of a sink is the first frame of the stack that is in `package:zoo_cells/` (a sink that a callback of the core library calls is the line of the callback). Keep a call of a sink on one line.
* `sinkAny` records the `toString` of the value: a struct that is given to it needs a `toString` that prints every field.
* A case may be `async` (the harness awaits it). An Isolate cannot be a case yet: its code runs in another isolate and the coverage does not see it.
* `In` (a source that is a parameter type, `rules.yaml` `param_type`) is a class with a final field `value`; the handler under test takes it: `void handle(In req) { // @source(tag)`.
* Flutter itself is not in the zoo: `build`, `onPressed` and the state store are written as small skeletons in the cell (`state/provider-store`, `dart/tearoff-instance` `vuln3`).
* A pubspec.lock is committed for `cells` and `exec`; CI resolves both with `dart pub get --enforce-lockfile`.

Stage 2 has 22 cells: **14 common forms**, which have the name of the Go cell (`call/direct`, `call/direct-in`, `call/depth3`, `call/multiret`,
`clos/inline`, `clos/returned`, `clos/to-own-helper`, `cont/slice` (a list), `cont/map-key-vs-value`, `state/global-direct`,
`state/handler-then-reader`, `state/receiver-field-across-entries`, `guard/sanitize-before`, `guard/one-branch`), written in Dart's own words, so that
the difference between the languages is read from one table; and **8 Dart forms** (`dart/patterns-record` (also the variables of a for-in),
`dart/tearoff-instance`, `dart/interp-tostring`, `dart/super-mixin-impl`, `state/provider-store`, `dart/flutter-field-read`,
`dart/flutter-override-arg`, `dart/flutter-route-args`), each with `// @na(go: <why>)` in its first lines: the form does not exist in the other language.
`TestZooDart` prints the parity line: the common cells in which Go finds the flow and Dart has a `@miss` (the `@miss` then says `@ref(..., PARITY)`; the
test fails without it), and the cells that only Dart has. A common form that the other language has not written yet is logged, not failed.
Some cells have more than one vulnerable case (`vuln2` ...): the forms that one review item (R-14, R-15, R-16) names are separate cases, because a `@miss` is a mark of one flow.

Flutter is not in the zoo: `packages/zoo_flutter` has a skeleton of what the cells need (`TextEditingController`, `RouteSettings`, `Navigator`,
`MethodChannel`, `State.setState`), real code of a few lines. What the user gives to the framework in a case is `typed(tag)` (the marker, which is **not**
a source of `rules.yaml`): the source of such a cell is the framework's API: a getter that is read (`TextEditingController.text`, `call_result`), a field that is read
(`RouteSettings.arguments`, `kind: field`) or the parameter of a method that an app overrides (`kind: param` with the symbol of each override; the rules
`match: implementations` and `extends` + `with_param_type` of an interface did not fire for a Dart class that extends or implements it, so each override is named,
and the position of such a source is the line of the `@override` of the method).

### What the Dart zoo found (the refs of its `@miss` and `@fp`)

| ref | what | cells |
| --- | --- | --- |
| `R-14` | the tear-off of an instance method swaps the receiver and the arguments: the flow is not found (`f = Printer('x').run; f(src())`, `stream.listen(c.onData)`, `Button(onPressed: form.submit)`), and the receiver's value is reported at the sink of the argument | `dart/tearoff-instance` (`vuln`, `vuln2`, `vuln3`, `decoy`) |
| `R-15` | a variable that a pattern (`var (a, _) = ...`) or a for-in (`for (var x in ...)`) declares is not a cell: a closure that assigns it is not seen | `dart/patterns-record` `vuln4`, `vuln7` (a pattern, an if-case, a switch case and a for-in whose variable is only read are found) |
| `R-16` | the implementation that comes from a mixin or a superclass of the class is not a candidate of a call through the interface; the interpolation `'$w'` does not call the `toString` (the result carries the whole object) | `dart/super-mixin-impl` (`vuln`, `vuln2`, `decoy`), `dart/interp-tostring` `decoy` |
| `KEYED-MAP` | the keys of a list or a map are not told apart (also a `Map<Object, _>` that a store keeps by provider) | `cont/slice`, `cont/map-key-vs-value`, `state/provider-store` (the decoys) |
