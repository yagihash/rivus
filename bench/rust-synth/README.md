# rust-synth: speed and resource scale for Rust (generated code)

`gen` writes a Rust crate of a given shape; `../go-synth/measure.py -lang rust` runs the extractor once and then
`rivus analyze -ir` (the part that is measured) on it, and prints the time, the peak memory, the number of findings
and uncertainties, and a hash of the set of finding IDs, so that two builds of the engine can be compared. There is no
answer key here: what is checked is that the result does not change. The accuracy is in `../rust-curated`. The plan of
the program is made by `../synthplan` (shared with `dart-synth`). The crate has no dependency, so that `cargo check`
(which the extractor runs) needs no network.

```sh
go run ./bench/rust-synth/gen -size M -out /tmp/rsynth-m              # S, M or L; flags override single parameters
source ~/.zshrc && (cd extractor/rust && cargo build)
export RIVUS_RUST=$PWD/extractor/rust/target/debug/rivus-rust
go build -o /tmp/rivus-before ./cmd/rivus
python3 bench/go-synth/measure.py run -lang rust -rivus /tmp/rivus-before -size M -runs 3 -out /tmp/rm-before.json
# ... change the engine, build /tmp/rivus-after, then the same, and
python3 bench/go-synth/measure.py compare /tmp/rm-before.json /tmp/rm-after.json
python3 bench/go-synth/measure.py check -lang rust -rivus /tmp/rivus-after -size S        # against expected/S.json
python3 bench/go-synth/measure.py snapshot -lang rust -rivus /tmp/rivus-after -size S     # only when the result is meant to change
```

The extractor is run once per `run` (1 to 3 s here), and its time is in `extract_s`; the time and memory are those
of `rivus analyze -ir` alone.

## What the generator makes

The same plan as `dart-synth` (see its README for the parameters: `-mods -funcs -depth -sccs -closure -relay -field
-capture -fan -sinks -sources`, the same sizes), written as `pub fn fN(s: String) -> String` in `m0.rs` ... with the helpers
`via_a(s, impl Fn)`, `via_relay`, `fire(s, impl FnMut)` and `Runner<F: Fn>` in `helpers.rs`. In Rust the closures given to
the shared helpers make **one big SCC** (the type of the closure parameter matches every closure), which is the part of
the engine that costs.

## What the engine took (2026-11-21, 12 cores, 24 GB; the build of the commit that adds this)

| size | functions (engine) | largest SCC | time | peak memory | findings / uncertainties |
| --- | --- | --- | --- | --- | --- |
| S | 210 | 153 | 0.35 s (0.35 to 0.73 over 3 runs) | 81 MB | 26 / 5 |
| M | 614 | 508 | 1.3 s | 418 MB | 128 / 5 |
| L | 1199 | 1030 | 13.2 s | 1.3 GB | 486 / 5 |

The time moves with the load of the machine (see `../go-synth/README.md`): compare two builds in the same
session, one after the other.

## Files

* `gen/main.go`, `gen/main_test.go`: the generator (and its test: deterministic, the seed matters, the digest of S).
  A change of the generator changes the code, which is a new benchmark: update the digest and `expected/`.
* `../synthplan`: the parameters, the sizes and the plan, shared with `dart-synth`.
* `expected/S.json`, `M.json`, `L.json`: the counts and the hash of the finding IDs when the snapshot was taken.
