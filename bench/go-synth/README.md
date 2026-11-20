# go-synth: speed and resource scale for Go (generated code)

`gen` writes a Go module of a given shape; `measure.py` runs `rivus analyze` on it and prints the
time, the peak memory, the number of findings and uncertainties, and a hash of the set of finding IDs,
so that two builds of the engine can be compared. There is no answer key here: what is checked is
that the result does not change (and how long it takes). The accuracy is in `../go-curated`.

```sh
# the code (the same parameters always write the same bytes)
go run ./bench/go-synth/gen -size M -out /tmp/synth-m        # S, M or L; flags override single parameters
cd /tmp/synth-m && rivus analyze -rules rules.yaml ./...

# time, memory, findings of one build
go build -o /tmp/rivus-before ./cmd/rivus
python3 bench/go-synth/measure.py run -rivus /tmp/rivus-before -size M -runs 3 -out /tmp/m-before.json

# ... change the engine, build it as /tmp/rivus-after, then
python3 bench/go-synth/measure.py run -rivus /tmp/rivus-after -size M -runs 3 -out /tmp/m-after.json
python3 bench/go-synth/measure.py compare /tmp/m-before.json /tmp/m-after.json     # IDENTICAL / DIFFERENT, time, memory

# against the snapshot in expected/ (what the result was when the snapshot was taken)
python3 bench/go-synth/measure.py check -rivus /tmp/rivus-after -size S
python3 bench/go-synth/measure.py snapshot -rivus /tmp/rivus-after -size S      # only when the result is meant to change
```

`measure.py` runs each run in a fresh process, so the peak memory is that run's own (`ru_maxrss`, which
is the peak of the resident set), and stops when two runs of the same binary give different findings
(they must not: the analysis is deterministic). `-flags "-timeout 900s"` passes flags to `rivus analyze`
(`-workers 1`, `-max-passes`, ...); `-dir DIR` measures any module that has a `rules.yaml`.

## How to use it when the engine changes

* The core (`internal/sg`, `internal/sites`, `internal/taint`) changed: run `go test ./...` (it includes
  `go-curated`), then **S and M**, before and after, with the **same machine state** (build both
  binaries first, run them one after the other, 2 or 3 times each). The result (`compare`) must be
  IDENTICAL for a change that is for speed. For a change that is meant to change the result, look at
  every finding that moved and say why in the commit, then `snapshot`.
* Gitea (350,000 lines, 10 to 16 minutes, 5 to 10 GB) and the other real projects are for a milestone.
* Sizes and what the engine took (the build of 2026-11-21, 12 cores, 24 GB, machine otherwise idle):

  | size | functions (engine) | largest SCC | time | peak memory | findings / uncertainties |
  | --- | --- | --- | --- | --- | --- |
  | S | 171 | 4 | 1.2 s (1.16 to 1.17 over 3 runs) | 126 MB | 25 / 25 |
  | M | 459 | 12 | 28.4 to 28.5 s (3 runs) | 807 to 818 MB | 113 / 87 |
  | L | 911 | 17 | 127 to 128 s (3 runs) | 3.2 GB | 176 / 174 |

  For a comparison, Gitea has 61,655 functions and takes 6 to 16 minutes and 5 to 15 GB (ADR-046), so
  these are small in functions but not in cost per function: the time per function is high on this
  shape (fields, SCCs, callbacks).
* The time moves with the load of the machine. The same binary on M took 28.5 s when idle (spread
  under 1 %) and 45 to 60 s when other analyses were running on the same machine; on a loaded
  machine the spread of one build is as large as 30 %. **Compare two builds in the same session**, run
  one after the other, 2 or 3 times each, and read a difference under about 10 % on an idle
  machine (30 % on a loaded one) as noise. Memory is steadier (S 124 to 127 MB, M 807 to 837 MB, even
  under load), but two big runs at once (Gitea, L) make the machine swap and then nothing here is
  measured. The finding IDs and the counts do not depend on the load, but a run that hits
  `-timeout` does (it is cut where it was): a run must converge (`stats.converged`) to be compared.

## What the generator makes (the parameters)

| parameter (flag) | what | S / M / L |
| --- | --- | --- |
| `-seed` | the seed of the generator (splitmix64: the same on every Go version) | 1 |
| `-pkgs` | packages; the functions are placed by level, so the imports have no cycle | 5 / 12 / 20 |
| `-funcs` | functions besides the SCCs and the implementations | 100 / 300 / 600 |
| `-depth` | levels of the call graph (a function calls lower levels, mostly the one just below) | 10 / 20 / 30 |
| `-sccs` | sizes of the groups of functions that call each other (a ring and chords, one level each) | 4,3 / 10,6,5,4 / 16,10,8,6,5,4 |
| `-ifaces`, `-impls` | interfaces in `model`, and the implementations of each (in the lower packages) | 2,4 / 3,6 / 5,8 |
| `-back` | percent of implementations that call the function that dispatches on their interface (a cycle across packages) | 50 / 30 / 30 |
| `-svc` | percent of the upper functions that call through an interface | 5 / 4 / 4 |
| `-fields` | fields of the struct every function takes (`model.Rec`) | 6 / 4 / 4 |
| `-fan` | calls of each function | 3 |
| `-deep` | percent of functions with a path of four fields (`r.Sub.Sub.Sub.F`), deeper than the engine follows (the source of `path_limit`) | 10 / 5 / 5 |
| `-with` | percent of calls made inside a closure given to a shared helper (`model.With`, as `db.WithTx` is used in Gitea): the helper calls every closure, so everything that uses it falls into one SCC | 2 / 0 / 0 |
| `-handlers` | functions in a route table (`map[string]handler`), called through a middleware chain (function values) | 10 / 40 / 80 |
| `-sinks`, `-sources`, `-sanitize` | percent of the functions with a sink, a source; of the sinks that sanitize first | |

Statements of the bodies: a plain call; a result stored in a field and another field read; `append`
to a slice and `strings.Join`; a map; a closure; the nested struct; a guarded call; a closure passed
to `model.With`. Every function has the shape `func F(r *model.Rec, s string) string`.

The size of the SCCs is what costs the most in the engine, and it is a parameter on its own: the
rings (`-sccs`), the closures given to a shared helper (`-with`), and the interface callbacks
(`-back`) are the three ways that Gitea's big SCCs are made, and each is set apart so that one can be
turned up alone. The sizes S, M and L are well below Gitea (see ADR-046 in `docs/DECISIONS.md` for the
measurements of Gitea this is taken from): the cost per function of the engine on this shape is high,
so the smaller code already takes what a real program of ten times the size does.

## Files

* `gen/main.go`, `gen/main_test.go`: the generator. The test checks that the code is deterministic,
  that the seed matters, and the digest of S (so that a change of the generator is on purpose).
  A generator change that changes the code is a new benchmark: update the digest in the test and `expected/`.
* `measure.py`: run / compare / check / snapshot.
* `expected/S.json`, `expected/M.json`: the counts and the hash of the finding IDs at the time of the
  snapshot (not the time). They are tied to the engine as it was then; after a change that moves the
  findings on purpose, take a new snapshot in the same commit.

## For Dart and Rust

The directory is named by language (`go-synth`). `dart-synth` and `rust-synth` go next to it, each with a
generator of its own and a `measure.py` that only differs in how the target is read (the IR of the
extractor goes in with `-ir`); the result format of `measure.py` (`findings`, `uncertainties`,
`ids_sha256`, `wall_s_*`, `max_rss_mb_max`) is the same for all.
