# dart-curated: accuracy regression for Dart (with answers)

A small Dart package (12 files and a second package, `packages/shared`, as a path dependency) of the
shapes that matter for closures and callbacks, written so that the baseline is there **before** the
helpers' summaries carry what a callback gives out (the plan for the big SCCs, the closures and the
callbacks being the part that changes). The sample of the extractor (`extractor/dart/test/sample`) tests
the extractor; this is a set of answers to compare two states of the engine.

The answers are in the source, in the way of `bench/go-curated` (same marks: `@source(tag)`, `@sink(tag)`,
`@sink(none)`, `@nosink(tag)`, `@miss(silent)` / `@miss(<kind>)`, `@fp`; see `../go-curated/README.md`):

```dart
final a = source();                         // @source(ret-a)
sink(transform(a, (x) => x));               // @sink(ret-a) @nosink(ret-b)
```

Most cases come in **pairs** (`..._a`, `..._b`) that use one helper from two callers, each with its own
source and sink: `@nosink(x-b)` on the sink of `a` says that the source of `b` must not get there (the
cross-talk of a helper that calls every closure of its type). `rules.yaml`: `source()` and `sink()` of
`lib/lib.dart`.

| file | what |
| --- | --- |
| `lib/helpers.dart` | the helpers: `transform` (closure returns), `fireAndForget`, `produce`, `run`, `twice`, a typedef, a generic one, a callback of a callback, `Function`, a closure kept in an object |
| `pass_closure.dart` | a closure given to a helper, two callers; the result, the argument, a captured value; block body; branches |
| `capture.dart` | a closure that writes a captured variable (or adds to a list); a relay through the parameter, through the capture, through both; a closure returned from a function |
| `iterables.dart` | `map`, `forEach`, `where`, `fold`, `reduce`, `expand`, `any`, `sort`, `Map.forEach`, `putIfAbsent` |
| `futures.dart` | `Future.then` chains, `catchError`, `whenComplete`, `async` closures, `Stream.listen` / `map` |
| `nesting.dart` | closures in closures, called twice, taking callbacks, generic helpers, `Function`, typedef, currying, composition, local recursion |
| `tearoff.dart` | tear-offs of methods, static methods, top-level functions, constructors, the sink; function-typed fields; tables; `this.method` as the callback |
| `callbacks.dart` | a closure that captures a value and is given to a function that calls it later: the widget callback (`Button(onPressed: ...)`, two, not mixed), `Timer.run`, `Timer.periodic`, `scheduleMicrotask`, `Future.delayed`, a write to a captured variable by a timer; the closure that uses its own parameter; a closure that is made and not called. Seven are `@miss` now (the capture is applied where the closure is made, C-7, `docs/DESIGN-capture.md`); the last two are quiet, and the last is the one C-7 would report |
| `classes.dart` | an emitter (callbacks kept in a list), `Lazy`, a builder, default and named callbacks, a getter that returns a function, an extension with a callback |
| `errors.dart` | `try` / `catch` / `finally`, typed catch, throw from a closure, rethrow, `Future.error`, `onError` |
| `across.dart` | the helper is in another package (`packages/shared`) |
| `pub_tar.dart` | the shape of dart-lang/pub's `extractTarGz` (`bench/dart-pub`, GHSA-q739): the name of a tar entry (a getter, the source), read in a `while (await reader.moveNext())` loop, joined into a path, and the `switch` on the type of the entry that gives the path to five file helpers (9 sinks, `pub_name`); all found |
| `dynamic_calls.dart` | a method of a `dynamic` value (`dart:core#dynamic.invoke`, covered by a model): DART-12, in the last section of [`../MISSES.md`](../MISSES.md); `@miss` 2 (`@miss(unmodeled_library_call)`, by a constructor), `@fp` 1 |

## What is not found yet (`@miss`) and what is reported wrongly (`@fp`)

Every `@miss` and `@fp` is classified by the cause (where in the engine the flow is lost, whether it leaves a trace in `uncertainties`, whether ADR-047 would fix it) in [`../MISSES.md`](../MISSES.md). Of the 34 `@miss`, 27 are `@miss(silent)` (no uncertainty in the function of the source or of the sink; a `dynamic_dispatch` with one candidate does not count) and 7 have a trace, `@miss(<kind>)` (`unresolved_callee` 3 and 1 more together with `unmodeled_library_call`, `dynamic_dispatch` 1, `unmodeled_library_call` 2: these two are the default constructors in `dynamic_calls.dart`, not the lost call); 12 are one cause (`CAP`: what a closure captured is lost when a helper calls it).

The first run (2026-11-21, the engine before the callback outputs go into the summaries) gave 74 flows found,
32 not found, 39 sinks quiet, 4 reported wrongly:

* **a closure that reads or writes a captured variable and is called by a helper** (`run(() => sink(a))`,
  `run(() { out = t; })`, `items.add(t)`, `nest-a/b`, `relay-c-a/b`, `twice-args`, `takes-a/b`,
  `across-cap`, `when-complete`): the captured value does not go into the closure that the helper calls
  (only a closure called where it is made, or one that gets the value as an argument, is followed).
* **a closure returned from a function** (`holder-a/b`, `curry`, `getter-fn`).
* **callbacks kept in an object and called later** (`emit`, `emit-two`, `across-hub`, `lazy`, `builder`),
  and `this.method` given as a callback (`bound-prefix`).
* **errors and futures** (`through`: a throw in a closure; `future-error`, `on-error`, `catch-error`,
  `completer`) and callbacks of the core library with two arguments or a closure of a different shape
  (`map-entries`: `Map.forEach`, `put-if-absent`).
* `@fp` (4): the result of a closure that returns a constant still carries the argument (`then-const`,
  `map-const`, `ret-clean`), and so does a tear-off that drops its argument (`tear-drop`).

Cross-talk of a helper between its callers (`@nosink`) is **not** seen today (the closures that are
given in place are matched to their callers), which is what a change must keep.

## Run it

```sh
export RIVUS_DART="mise exec dart@3.13.6 -- dart"
go test ./cmd/rivus -run TestCuratedDart -v     # about 6 seconds, part of "go test ./..." when RIVUS_DART is set
```

The test runs `dart pub get --offline` first if `.dart_tool` is missing (the only dependency is the path
package, so it needs no network), runs the extractor, then `rivus analyze -ir`, and compares the findings with the
annotations. `.dart_tool` is not committed. A flow that is found now makes the test fail with "remove
@miss"; a flow that is new, or an `@fp` that is gone, fails too. So does a trace that has come to a
`@miss(silent)` (write its kind: a gain) or gone from a `@miss(<kind>)`. The findings with no step in
the function of their source are counted (`offSourcePath`, BENCH-2): 0 here, a ceiling.
