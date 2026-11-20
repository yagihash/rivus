# rust-curated: accuracy regression for Rust (with answers)

A small crate (10 files, and the crate `shared` as a path dependency) of the shapes that matter for closures and
callbacks: `Fn`, `FnMut` and `FnOnce` parameters, `impl Fn` and `&dyn Fn` and `Box<dyn Fn>`, function
pointers, `map` and `and_then` of `Option` and `Result`, `?`, closures kept in structs, `&self` methods
given to a helper. It is the baseline **before** the helpers' summaries carry what a callback gives out; the
sample of the extractor (`extractor/rust/test/sample`) tests the extractor, this is a set of answers to compare
two states of the engine.

The marks are those of `bench/go-curated` (`@source(tag)`, `@sink(tag)`, `@sink(none)`, `@nosink(tag)`,
`@miss`, `@fp`; see `../go-curated/README.md`), and one more, for this language:

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

## What is not found yet (`@miss`) and what is reported wrongly (`@fp`, `@noisy`)

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

## Run it

```sh
source ~/.zshrc && (cd extractor/rust && cargo build)
RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust go test ./cmd/rivus -run TestCuratedRust -v   # about 3 seconds
```

The test runs the extractor on this crate (it runs `cargo check`, which makes `target/`, not committed) and
`rivus analyze -ir`, and compares the findings with the annotations. The crate compiles with plain `cargo check`.
