# rust-curated: accuracy regression for Rust (with answers)

A small crate (10 files, and the crate `shared` as a path dependency) of the shapes that matter for closures and
callbacks: `Fn`, `FnMut` and `FnOnce` parameters, `impl Fn` and `&dyn Fn` and `Box<dyn Fn>`, function
pointers, `map` and `and_then` of `Option` and `Result`, `?`, closures kept in structs, `&self` methods
given to a helper. It is the baseline **before** the helpers' summaries carry what a callback gives out; the
sample of the extractor (`extractor/rust/test/sample`) tests the extractor, this is a set of answers to compare
two states of the engine.

The marks are those of `bench/go-curated` (`@source(tag)`, `@sink(tag)`, `@sink(none)`, `@nosink(tag)`,
`@miss(silent)` / `@miss(<kind>)`, `@fp`; see `../go-curated/README.md`), and one more, for this language:

* `@noisy(N)` on a sink line: **N sources of other tags reach it, wrongly**, and no `@nosink` names them.
  Rust matches a call of a closure parameter to every closure of that type, so the closure that holds a sink
  is reached by whatever any caller of the helper gives (`run(|| sink(&a))` gets 6 foreign flows;
  `fire_and_forget(a, |x| sink(&x))` 40). They are counted, not named, and the test fails when the number
  changes (fewer is an improvement: write the new number). The sum is in the log of the test.

| file | what |
| --- | --- |
| `src/helpers.rs` | `transform`, `fire_and_forget`, `produce`, `run`, `twice` (FnMut), `apply_to` (generic), `via_inner`, `call_dyn`, `call_boxed`, `holder`, `Runner<F>`, `sink_with` |
| `pass_closure.rs` | a closure given to a helper, two callers; result, argument, capture, `move`, branches, FnMut called twice, `&dyn Fn`, `Box<dyn Fn>`, the sink as the callback's |
| `capture.rs` | a closure that writes or pushes to a captured variable; relays through the parameter, the capture, both; closures returned (`impl Fn`, `Box<dyn Fn>`) |
| `iters.rs` | `map`, `for_each`, `filter`, `fold`, `flat_map`, `zip`, `Option::map` / `and_then` / `unwrap_or_else` / `map_or`, `Result::map` / `map_err`, `sort_by`, `or_insert_with` |
| `nesting.rs` | closures in closures (3 levels), generic helpers, a callback of a callback, `Box<dyn Fn>` chosen by a condition, currying, composition, a recursive function with a callback |
| `fnvalues.rs` | fn items and fn pointers (`fn(String) -> String`), a closure as a pointer, the sink as a pointer, `String::clone` as the callback, tables of boxed functions, a function that returns a pointer |
| `structs.rs` | a closure kept in a struct, an emitter (`Vec<Box<dyn Fn>>`), `&self` methods given to a helper, a trait with a callback parameter and two implementations |
| `errors.rs` | `Result`, `?`, `map_err`, `match`, a closure with `?`, a helper that takes a closure returning `Result`, `or_else`, `and_then` |
| `across.rs` | the helper is in another crate (`shared`) |
| `parseable.rs` | the shapes of the parseable advisories (`bench/parseable`): `web::Query` to the Location header, `web::Json` to the path of `put_object` (a trait with two implementations), `web::Form` stored in a global `OnceLock<RwLock<HashMap>>` and posted to later (**not found**, `pb_target`, `@miss(silent)`: the SSRF of GHSA-64vg that the benchmark misses too), `web::Path` to `Client::get`. The mark of each source is on the line of its `into_inner` (the place where the value of a `param_type` is first used) |
| `fnself.rs` | `Fn` / `FnMut` called through `&self`, two callers each, own signatures so as not to change other counts: the acceptance of ADR-047 (draft 2), RUST-13; `@fp` on all, to go |
| `narrow.rs` | the candidates of `Fn::call` narrowed to the closures that flow to the receiver (ADR-079) and the forms where narrowing would lose a flow, so every candidate stays: a closure that is the result of a call, a closure in a field, a relay; and the edge of the limit (8 callers narrow, 9 do not) |
| `main.rs` | a `main`: the program starts somewhere, so what the library has that is `pub` is read as a closed world (`closed_world_entry`). The library with no `main` is `../rust-curated-lib`, and one with a function that another program can call by its symbol is `../rust-curated-ffi` |
| `modelled.rs` | a trait method of the crate that rules.yaml models, called through a generic: the model is applied and the implementation is read as well (ADR-075); `@miss` 0, `@fp` 1 |

**After the narrowing (ADR-079):** the `@fp` of the first fixtures are 7 (from 18; the six of `fnself.rs`, which ADR-047 meant to remove, are gone) and the `@noisy` flows 84 (from 152; three places). What is left comes from the helpers whose receiver is not followed (a field, a `Box<dyn Fn>`, the result of a call).

## Closures that capture, and where they are called (`src/callbacks.rs`)

A closure that captures is not a `make_closure` in Rust: it is a struct of its environment (an `alloc`
of the type `{closure@file:line:col}` and a `field_store` for each captured value), and the closure is
a function of that struct, called by `Fn::call` (the candidates are the closures with that parameter
list). So the shape of C-7 (`docs/DESIGN-capture.md`: the captured values are applied where the closure is made) does
not apply to Rust as it is. `callbacks.rs` has four forms that the engine finds now through the
candidates of `Fn::call`: a closure returned by a function, a closure in a closure, a closure that is
made and not called (quiet), a closure that uses its own parameter (quiet). Each has two parameters of
types of its own, as in `fnself.rs`: a closure with the parameters that the others have (a closure with none,
as the one of `thread::spawn`) would be a candidate of their calls, and would change their `@noisy` counts.
Three forms where the closure is only given away are **not found** now (`@miss`, so the reading side is not found everywhere): `thread::spawn` (`@miss(unmodeled_library_call)`), a `Box<dyn Fn>` in a field that another function calls, `Vec::retain`. The closure of `spawn` and of `retain` is a candidate of the calls of closures with the same parameters, so other sources reach their sinks (`@noisy`). The writes to a captured variable (`capture.rs`, `@miss(silent)`) are not found because of ENV-PTR, and
applying the closure where it is made would not change that.

## What is not found yet (`@miss`) and what is reported wrongly (`@fp`, `@noisy`)

Every `@miss`, `@fp` and `@noisy` is classified by the cause in [`../MISSES.md`](../MISSES.md). Of the 13 `@miss`, all 13 are `@miss(silent)` (no uncertainty in the function of the source or of the sink; none has a trace); 7 are one cause (`ENV-PTR`: a write through a pointer kept in a closure's environment is not seen), none is fixed by ADR-047. All 16 `@noisy` places (183 flows; the text below says 17) and 12 of the 14 `@fp` are one cause (`XTALK`: the candidates of a closure call are all closures of its type).

The first run (2026-11-21, before the callback outputs go into the summaries): 112 flows found, 11 not found,
42 sinks quiet, 14 reported wrongly, 183 flows of other sources into 17 noisy sinks.

* `@miss` (11): **a closure that writes or pushes to a captured variable** (`write-a/b/param`, `push-list`,
  `late`, `for-each-cap`, `across-cap`), and **callbacks kept in a `Vec<Box<dyn Fn>>` and called by `emit`**
  (`emit`, `emit-two`, `across-hub`), and `across-each` (`FnMut` of another crate with the sink in the closure).
* `@fp` on `@nosink` (8): the two callers of one helper (`fire_and_forget`, `run`, `sink_with`, `relay_capture`)
  reach each other's sink, because the closure called in the helper is any closure of its type.
* `@fp` on `@sink(none)` (6): a closure that returns a constant still carries the argument (`ret-clean`, `produce`,
  `map-const`, `and_then` with a constant), an fn item that drops its argument (`fn-drop`), the second emitter.
* `@noisy` (183 flows): the cross-talk above, for the callers that no pair names.

The review items RUST-13 / RUST-14 and their fixtures are in the last section of [`../MISSES.md`](../MISSES.md). `fnself.rs` and `modelled.rs` were added after the first run above: now 123 flows found, 12 not found, 45 sinks quiet, 22 reported wrongly, the same 183 noisy.

## Run it

```sh
source ~/.zshrc && (cd extractor/rust && cargo build)
RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust go test ./cmd/rivus -run TestCuratedRust -v   # about 3 seconds
```

The test runs the extractor on this crate (it runs `cargo check`, which makes `target/`, not committed) and
`rivus analyze -ir`, and compares the findings with the annotations. The crate compiles with plain `cargo check`.

The `@miss(silent)` / `@miss(<kind>)` marks are checked from both sides (a trace that has come to a silent one, or gone from one with a
kind, fails the test). The findings with no step in the function of their source are counted
(`offSourcePath`, BENCH-2): 0 here, a ceiling.
