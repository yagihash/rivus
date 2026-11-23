# Notes on bench/heavy-ledger.tsv

The ledger is written only by the tools (`make bench-passline`, `make bench-deps`; `NO_LEDGER=1` adds no row). Read it with
`python3 bench/heavy_ledger.py [--kind passline|deps] [--target memos] [--last N]`; the margin is computed there, not stored.
How to measure and what the lines mean: [passline-baseline.md](passline-baseline.md). The light benches are in [LEDGER.md](LEDGER.md).

- 2026-11-23, fe7740d6: memos passed 3 times (48 to 50 s, 5.99 to 6.15 GB, converged, detected, `quiet_after_fix=no` recorded only). These runs were made before the ledger existed; the output is not kept.
- The 3 rows of 2026-11-23 at `6bee840` (go-s3, gcp, dart-gcp) were moved from `bench/deps-ledger.tsv` (removed). That file had columns that the ledger does not have; they are kept here: max_rss_mb (end-of-run peak of the main process, not the tree sum, so `peak_tree_gb` is empty) 12449, 4012, 1530; functions 5845, 20810, 4593; uncertainties 47, 143, 738. `result`: timeout, finished, finished.
- The 3 rows of 2026-11-23 at `dac8bcb6` (go-s3, gcp, dart-gcp) and the 3 old rows at `6bee840` differ in both the way of measuring (the tree sum, not the main process) and the rivus commit: do not compare them.
- The column `live_heap_max_bytes` is a guide on the upper side: the largest amount that the GC marked live (`/gc/heap/live:bytes`). It counts what was allocated while the GC was marking, so in a phase that allocates fast (the summary phase) it is larger than what is really live. It is not the real live size. `live_margin_gb` in the table (limit minus this value) is therefore on the low side. Correction of 2026-11-24 (memos, plain binary, one run: load phase 4.34 GB against 4.27 GB really live; summary phase 4.76 GB against 2.21 GB really live); see ADR-121.
