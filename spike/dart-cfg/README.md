# Dart spike: can a CFG be built from `package:analyzer`?

Throwaway code (docs/ARCHITECTURE.md, "Dart スパイク"; ADR-011, ADR-037). It answers
three questions before the IR is designed. The code is not meant to be kept.

## Verdict

**Feasible.** The CFG of every construct the brief names as hard comes out right
(`dart test`: 20 tests, all passing), package and generated code resolution works,
and nothing found calls for a change of architecture. What it did find are
decisions the IR has to take, listed below.

Run it:

```sh
cd spike/dart-cfg
dart pub get            # (with the SDK at hand; here: mise exec dart@3.13.6 -- dart ...)
dart test
dart run bin/cfg.dart samples/cfg/constructs.dart --dot out/dot   # DOT files to look at
dart run bin/resolve.dart                                          # monorepo and generated code
```

## 1. Can the CFG be built?

`lib/cfg.dart` (about 550 lines of builder) builds the graph from the resolved AST for:

| construct | result |
|---|---|
| `async` / `await` (in sequence, in conditions, in loops) | an `awaitResume` edge to a fresh block per `await`; no new instruction (ADR-003 holds) |
| `await for`, `yield` | the same edge at each step of the loop and at each `yield` |
| `try` / `catch` / `finally`, with `return`, `break`, `throw`, `rethrow` leaving it | correct; `finally` is **copied** once per way out (4 copies for the test function) |
| nested `try`, `try` around `await` | correct; an exception at an `await` reaches the catch clauses |
| labeled `break` / `continue`, `while`, `do`, `for`, for-in | correct targets, back edges |
| `switch` with patterns, `when` guards, switch expressions | structure right (match / fail edges, guard blocks); patterns themselves not lowered |
| `&&`, `\|\|`, `!`, `?:`, `??` | control flow, not operators (expression level lowering) |

Every function: no dead block, and every block reaches the exit.

## 2. Does package resolution work in a monorepo?

Yes. One `AnalysisContextCollection` over the monorepo gives a context per package and
one for the root, taken from each `package_config.json`. A call in package `b` to a
function of package `a` resolves to its declaration in `a/lib/a.dart`.

## 3. Is generated code visible?

Yes, as long as it exists on disk (`build_runner` has run, which is a precondition
of the workflow, not of rivus). `part 'user.g.dart'` is part of the library: a call to
`_$UserFromJson` resolves to its declaration in `user.g.dart`.

## What the IR has to decide (the list the spike was for)

1. **Exception edges and blocks.** An exception edge per *block* is wrong once a block
   holds two calls, or code the handler does not cover (the `finally` copy that follows
   the last call of a `try` body). The spike first got exactly that wrong. The fix is
   the one MIR uses: inside a protected region a call that may throw **ends its block**,
   with one edge for the exception and one for the normal continuation. Outside any
   handler no exception edge is drawn (an exception that leaves the function is not a
   flow inside it). This resolves the "call has an exception edge" sentence of
   ARCHITECTURE.md 2.4 in favour of the terminator form for calls in a try region.
2. **`finally`** (open item 4 of the review): the spike **duplicates** the finally block
   per way out, so the IR needs no notion of `finally`. The cost is a copy per exit
   (4 for the test function). The consequence for rivus: the copies have the same source
   position, so site enumeration must count a call in a duplicated block **once** (ID,
   ordinal and fingerprints of ADR-028 would otherwise change with the number of exits).
   Every instruction therefore needs a source position, and the extractor must mark
   copies, or the core must deduplicate by position.
3. **Expression level lowering.** `&&`, `||`, `?:`, `??` and switch expressions become
   blocks and need a local that holds the value at the join (the mutable locals of
   ADR-003 fit; the core makes the φ). The extractor needs temporaries.
4. **Patterns** are the largest piece not done: a pattern is a type test plus field reads
   plus bindings. The IR already has `type_assert` and `field_load`; the work is in the
   extractor. Exhaustiveness is not reasoned about: the edge "no case matched" stays.
5. **Suspension** is an edge, as decided. `yield` and the elements of a `Stream` are not
   a flow inside the function: they need a model (or a call graph that links producers
   to the `await for` that consumes them).
6. **Closures and local functions** are graphs of their own, as in Go.
7. **Constructor calls** are `InstanceCreationExpression`, not `MethodInvocation`: the
   extractor has to cover every kind of invocation node.

## Things that cost time

- **`package:analyzer` changes its API between versions.** Version 14.7 differs from
  what most documentation shows: `ClassDeclaration` has no `name` (it has a fragment),
  arguments are a sealed `Argument` (not `Expression`/`NamedExpression`), `for` headers
  have a `ForPartsWithPattern`, the element model is version 2 (`firstFragment`).
  The extractor must pin the analyzer version, and the SDK that goes with it.
- The Dart toolchain was not on the path here (`mise` shim without a version). It was
  reached with `mise exec dart@3.13.6 -- dart ...`; the user's configuration is untouched.

## Not examined

- **The call graph.** `pkg/vm` (type flow analysis) was not looked at: this is the
  next risk of Phase 4, with the performance of resolving a large Flutter project.
- Calls on `dynamic` receivers (the element is `null`): how common they are in real
  Flutter code is not known.
- Anything about build time or memory: the samples are tiny.
