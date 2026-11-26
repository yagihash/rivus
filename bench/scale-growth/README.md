# scale-growth

How the live memory of each stage grows when the target gets bigger. It is an investigation, not a pass/fail
check: nothing goes to `bench/heavy-ledger.tsv`.

Why: `-max-memory` is decided by the load stage (on memos about 4.3 GB after a forced collection; the summary stage is about
2.2 GB). The point of writing solved parts to disk is to cut what is held from the size of the target. For that we need
the **slope**: how much each stage's live memory grows when the target doubles. This tool measures it on the
synthetic targets of `bench/go-scale/gen`.

```
python3 bench/scale-growth/scale_growth.py [-scales 1,2,4] [-profile mixed|ops|funcs] [-workers 1] [-rivus BIN] [-cap-gb 8]
```

It builds `rivus` and `gen` from this tree into a temporary directory (or uses `-rivus`), makes each target under the
temporary directory, takes the heavy-run lock (`bench/heavylock.py`) for each run, and watches the run with the same
watchdog as `bench/deps_bench.py` (`run_watched`: the memory of the process tree (the footprint on macOS, the sum of RSS elsewhere: `mem_basis`) above `-cap-gb`, default 8 GB, or the
wall time above `-limit`, kills this run's own process group). The table goes to standard output; the JSON goes to
`$BENCH_OUT/scale-growth-<profile>.json` (with the commit and the load average).

## What is measured, and how

Nothing in rivus is changed. The tool runs `rivus analyze -v -no-deps -workers 1` with `GODEBUG=gctrace=1` and reads stderr:

| column | source | exact? |
| --- | --- | --- |
| `load_mb` | the `-v` line of the last phase before `taint` (`graph`, or `graphdb`): `-v` calls `runtime.GC()` and then prints `HeapAlloc` | yes (MB, rounded down) |
| `summary_mb` | the largest `Z` of `X->Y->Z MB` of the collections rivus did not force, between that phase and the `taint` line | **upper bound**: Z is what the mark found, and it includes what was allocated while the mark ran |
| `functions`, `summary_elems`, `summary_steps` | `stats` of the report | yes |
| `wall_s`, `cpu_s` | the watchdog (`wait4`) | |
| `memory_peak_bytes`, `gc_cpu_fraction` | `summary` of the report | |

The summary stage has no forced collection inside it (the `taint` phase line comes after the summaries are dropped),
so its value is the gctrace one. Repeated runs at GOGC 25 and 100 gave 851 to 932 MB at the same target
(about 10 percent), so read a single value as plus or minus 10 percent, and trust a slope more when the scales are far
apart. `-workers 1` is the default because other workers allocate more during a mark. If this is not good enough, the
exact value needs a forced collection at the end of the summaries; that means a small, off-by-default hook in rivus
(a `memtrace.Stage(name)` call at 4 places, active only with an environment variable).

The growth is printed per step (1x to 2x, 2x to 4x) as the ratio and as the exponent `log(ratio)/log(scale ratio)`:
1 is linear in the scale, 2 is square.

## Why not `/gc/heap/live:bytes`

`summary.live_heap_max_bytes` (from `/gc/heap/live:bytes`) is too big in the summary stage: it counts what was allocated
between the marks (measured on 2026-11-24). The tool reports it in the JSON only for comparison, and does not use it.

## The profiles

All use `stagefield` (the shape of memos' AWS SDK: stages as an interface field, one strongly connected component
whose summaries grow with the square of the operations; see `bench/go-scale/README.md`). The knobs at scale k:

| profile | what grows | base (k = 1) |
| --- | --- | --- |
| `mixed` (default) | operations, ordinary functions and packages, all by k | ops 40, stages 4, fields 8, funcs 2000, pkgs 20 |
| `ops` | only the operations (the component and the summaries) | ops 40, funcs 2000 |
| `funcs` | only the ordinary functions (the load; no big component) | funcs 2000 |

`mixed` is the nearest to a real product (more code and more operations together). Reading the three together tells which
part of the slope comes from the load (`funcs`), which from the summaries (`ops`). The base values are in
`PROFILES` in the script. At 1x the run takes about 2 s and 0.25 GB; at 2x about 4 s and 1.2 GB; 4x is
roughly 16 times the elements of 1x for `mixed`, so check `-cap-gb` and the free memory before it.

## Test

`python3 bench/scale-growth/test_scale_growth.py` (fake stderr and numbers; nothing is run). It is in CI.
