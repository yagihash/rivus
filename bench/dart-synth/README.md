# dart-synth: speed and resource scale for Dart (generated code)

`gen` writes a Dart package of a given shape; `../go-synth/measure.py -lang dart` runs the extractor once and
then `rivus analyze -ir` (the part that is measured) on it, and prints the time, the peak memory, the
number of findings and uncertainties, and a hash of the set of finding IDs, so that two builds of the engine can
be compared. There is no answer key here: what is checked is that the result does not change. The accuracy is in
`../dart-curated`. The plan of the program is made by `../synthplan` (shared with `rust-synth`).

The package has no dependency, and the generator writes `pubspec.yaml` and `.dart_tool/package_config.json` too,
so that it is read without `pub` and without a network.

```sh
go run ./bench/dart-synth/gen -size M -out /tmp/dsynth-m              # S, M or L; flags override single parameters
export RIVUS_DART="mise exec dart@3.13.6 -- dart"
go build -o /tmp/rivus-before ./cmd/rivus
python3 bench/go-synth/measure.py run -lang dart -rivus /tmp/rivus-before -size M -runs 3 -out /tmp/dm-before.json
# ... change the engine, build /tmp/rivus-after, then the same, and
python3 bench/go-synth/measure.py compare /tmp/dm-before.json /tmp/dm-after.json
python3 bench/go-synth/measure.py check -lang dart -rivus /tmp/rivus-after -size S        # against expected/S.json
python3 bench/go-synth/measure.py snapshot -lang dart -rivus /tmp/rivus-after -size S     # only when the result is meant to change
```

The extractor (about 10 s on every size: it starts the Dart analyzer) is run once per `run`, and its time is in
`extract_s`; the time and memory of the runs are those of `rivus analyze -ir` alone.

## What the generator makes

Every function has the shape `String fN(String s)` and calls `-fan` functions of lower levels, mostly the level
just below. A call is plain, or goes through a closure given to a shared helper (`-closure` percent), which is how
the big SCCs of real programs are made: the helper calls every closure, so everything that uses it can end up in one SCC.

| parameter (flag) | what | S / M / L |
| --- | --- | --- |
| `-seed` | splitmix64: the same on every Go version | 1 |
| `-mods` | files; the functions are placed by level, so the imports have no cycle | 5 / 12 / 20 |
| `-funcs` | functions besides the SCCs | 100 / 300 / 600 |
| `-depth` | levels | 10 / 20 / 30 |
| `-sccs` | sizes of the groups of functions that call each other (a ring and chords; the call back is guarded and never made) | 4,3 / 10,6,5,4 / 16,10,8,6,5,4 |
| `-closure` | percent of calls through a closure given to a shared helper (`viaA(v, (x) => f(x))`) | 30 |
| `-relay` | of those, percent through a second helper (`viaRelay`, which gives the closure on) | 30 |
| `-field` | of the rest, percent where the closure is kept in a field (`Runner(f).go(v)`) | 25 |
| `-capture` | of the rest, percent that capture a local (`(x) => f(x + v)`) or write one (`fire(v, (x) { w = f(x); })`) | 40 |
| `-fan`, `-sinks`, `-sources` | calls of each function; percent of functions with a sink, a source | 3; 6 / 5 / 4; 4 / 3 / 2 |

## What the engine took (2026-11-21, 12 cores, 24 GB; the build of the commit that adds this)

| size | functions (engine) | largest SCC | time | peak memory | findings / uncertainties |
| --- | --- | --- | --- | --- | --- |
| S | 210 | 4 | 0.13 s | 21 MB | 4 / 19 |
| M | 614 | 10 | 0.14 s | 31 MB | 16 / 45 |
| L | 1199 | 16 | 0.18 s | 40 MB | 39 / 109 |

This is cheap, and the findings are few, on purpose of the engine as it is: a closure that captures a variable and is
called by a helper loses the captured value (see `../dart-curated`), so most flows that go through the helpers are not
found, and the SCCs stay small. When the callbacks' outputs go into the summaries, the findings and the SCCs grow:
that is the change this benchmark is for (expect `compare` to say DIFFERENT, and read the new findings).

## Files

* `gen/main.go`, `gen/main_test.go`: the generator (and its test: deterministic, the seed matters, the digest of S).
  A change of the generator changes the code, which is a new benchmark: update the digest and `expected/`.
* `../synthplan`: the parameters, the sizes and the plan, shared with `rust-synth`.
* `expected/S.json`, `M.json`, `L.json`: the counts and the hash of the finding IDs when the snapshot was taken.
