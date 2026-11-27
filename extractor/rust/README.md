# rivus for Rust

(The crate `rivus` on crates.io is this extractor: it installs the command `rivus-rust`. The
analysis engine `rivus` itself is a Go program, <https://github.com/yagihash/rivus>.)

```sh
rustup toolchain install nightly-2026-11-26 --profile minimal -c rustc-dev -c llvm-tools -c rust-src
cargo +nightly-2026-11-26 install rivus     # the command: rivus-rust
```

# The Rust extractor of rivus

**Reading a project builds it.** The extractor runs `cargo check` on the workspace, so the
`build.rs` of the project and of its dependencies, its proc macros, and its `.cargo/config.toml`
run with your rights. Use it on code you trust, or in a throwaway environment (a container or a VM,
the dependencies fetched beforehand, no network), as the section "信頼できないコードを解析するとき" of the
[README of rivus](../../README.md) says. Using `rivus-rust` by itself does not ask for `-lang rust`
as `rivus analyze` does: running the command is the request. The build also sees where the IR is
written (`RIVUS_IR_DIR`): a build script of the project can add to the fragments, or remove from them,
so what the target built is not only what is analysed, it can also change what the analysis says.

`rivus analyze -lang rust` runs this extractor under the sandbox of macOS by default (nothing is run on another system);
`-unsafe-host-exec` runs it on the host. How, and what the sandbox does not protect: [docs/rust-isolation.md](../../docs/rust-isolation.md).

Reads Rust through the compiler: MIR, by way of `rustc_public` (the stable interface of
`rustc`'s MIR), and writes the neutral IR (`docs/ARCHITECTURE.md` 2) as JSON Lines. The
core reads it with `rivus analyze -ir`.

```sh
cd extractor/rust
cargo build                                   # needs the pinned nightly (rust-toolchain.toml)
target/debug/rivus-rust path/to/workspace > workspace.jsonl
rivus analyze -ir workspace.jsonl -rules rules.yaml
```

Only the library and the binaries are read: the code that is shipped. Tests, examples and
benchmarks are not even compiled, so `#[cfg(test)]` code cannot get in. `--include-tests`
(also spelled `--tests`, like `rivus -tests` for Go; before the directory) reads them too (`cargo check --all-targets`).
The stream says how many targets (tests, examples, benchmarks) and how many `[dev-dependencies]`-only crates were left out (`tests_skipped`, ADR-063).

It builds the workspace with `cargo check`, running itself in place of rustc for the crates
of the workspace (`RUSTC_WORKSPACE_WRAPPER`): rustc compiles them as usual, and the driver
writes the IR of each crate on the way. The code has to compile. The dependencies are read too
(the driver stands in for rustc for every crate; cargo tells the members of the workspace apart
by `CARGO_PRIMARY_PACKAGE`): their functions are marked `dep`, and `rivus analyze` keeps the ones
the code reaches, so a flow goes through a dependency instead of being guessed at. Proc macros are
not read, nor is `std` (the models stand for it). `--no-deps` reads the members only: calls into a
dependency are then unmodeled library calls, until a model says what they do. `--body-replaced <crate>` (repeatable; `rivus analyze` passes the crates that the rule files and `-body-replaced` declare: `actix-web` and `actix_web` are the same crate) lowers none of the bodies of that dependency: the stream has a `body_not_read` record (`unit`, `functions`) in place of its functions. `cargo check` still compiles the crate, so the time and the memory of the build do not fall; the IR is smaller. The candidates of a
call through `dyn Trait` (or a type parameter) are the impls of every crate that was read: the crate that
is compiled does not know the others, so the stream is merged first and the candidates are completed
then (the same for `Drop::drop` of a type of another crate). Closures are the exception: the candidates
of a call of a function value are the closures of the crate that makes the call.

The extractor is pinned to one nightly, written in `rust-toolchain.toml` (`build.rs` hands it to the code):
`rustc_public` is unstable, and moves with the compiler. To move it, change that file and
the install hint in `cmd/rivus/lang.go` (a test checks they agree), and fix what the new compiler
broke. The extractor checks at start that the nightly is installed, and says how to install it if not.

Test: `RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust go test -run TestRustExtractor ./cmd/rivus`.
The sample is `test/sample`, with `@source(tag)` and `@sink(tag)` annotations.

**On a Mac:** if linking fails with `tapi error: malformed file ... unknown architecture`,
the newest SDK of the Command Line Tools is newer than its linker. Set
`SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX26.5.sdk` (an older one).

## Symbols

The rules name Rust things by the path of the function: `std::fs::read_to_string`,
`std::process::Command::new`. A trait method is named by the trait, not by the impl
(`std::ops::Deref::deref`, `std::iter::Iterator::next`): a rule on it applies to every
impl. Generic parameters of the impl are left out (`std::option::Option::unwrap`).
**The receiver is the first argument**: a model says `arg0` for `self` (Rust has no
receiver apart from the arguments), and a sink's `args: [1]` is the first argument after
`self`.

## How MIR is lowered (what is decided)

- A local whose address is taken (`&x`) lives in a cell (`alloc`, `load`, `store`); the
  others are IR locals. A place with a projection is read and written through
  `field_load`, `field_store`, `index_*`, `load`, `store`; `(*p).f` is one access.
- An enum variant's field is named `Variant.field`, a tuple's and a closure's fields `0`,
  `1`, ...
- A call is resolved by `Instance::resolve`: a function with a body in the workspace is a
  static call (`func` is the name of the definition, without type arguments; `symbol` the path or the trait method); one without
  is an external static call (a model says what it does); one that cannot be resolved
  (a type parameter, a trait object) is `dynamic`, with the impls of the workspace as
  candidates.
- A closure is called through `Fn::call` and the like: resolved by its type, with the tuple
  of arguments taken apart.
- Unwinding (cleanup blocks, `drop`, `assert`) is not drawn: a panic is not a flow inside
  the function. A call that does not return ends in `unreachable`.
- Positions are lines and columns (rustc_public gives no byte offsets): the offset in the
  stream is `line * 100000 + column`.

## Not done

- `async` works where the future is awaited in the code that is read (the coroutine is
  resolved by `Future::poll`; `std::future::IntoFuture`, `Pin` are modeled); the executor
  that runs the futures of a framework is not. Generics are analysed as written (not per
  instance), `unsafe` pointer arithmetic is not followed, inline assembly is not read,
  `dyn Trait` methods have the impls of the workspace as candidates only.
- The text of the arguments of a call (the stream has the call as written only), and the
  body in the token-normalized form.
- Models: `internal/model/builtin/rust-*.yaml` cover the common part of `std`. Others
  (`reqwest`, `sqlx`) have their sinks in `rust-std.yaml`, and `rust-axum.yaml` and `rust-web.yaml` have sources for
  `axum` and some other web frameworks (ADR-063 left the question of which frameworks are built in
  open; see `docs/PRINCIPLES.md`, section 3). actix-web is built in
  (`internal/model/builtin/rust-actix-web.yaml`).
