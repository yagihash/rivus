**Unverified rows: 0** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main; the status of each row is in `bench/backtest-rows.json`). It is counted again at every change of that file.

No row is unverified.

Row 8: the first table of this page named `scale/selfcopy` and `scale/all` for it, and a good version "see the ledger"; the ledger has no such section and both units are green on the bad build. It is judged on `curated-go` (the decoy `api/restore.go`), and the bad build is rebuilt from `5dfec7f` and `bench/backtest-patches/8-no-plan-d.patch`.
Row 10: the good version was a branch commit (`6ddee62`); the same patch is `a1e2fb7` on main, and the decoys were a `.pending` file until the file was enabled.

Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.

**The day the number reached 0: 2026-11-21** (the day the last of the rows was verified; a row that is added later starts as unverified).

Holes: only the pairs of rows 6, 7 (layer C) have compare runs so far, and row 8 and 10 are judged by `TestCuratedGo`; the rows of A (go-scale) are in BACKTEST.md. Wall times were taken while the machine was shared (read them as a guide; the counts are exact).

# Ledger: version x bench x metric

Made by `bench/ledger.py` from the runs of `bench/compare.sh` (see the header of that script). Inputs `head`: the engine of the row was run on the inputs of the reference tree (`compare.sh --fixed-inputs`, bench/BACKTEST.md); `own`: on the inputs of its own version.
The difference is against the reference engine `de627ce` on the same inputs: `+added -removed` findings (counts only). A `-` is a hole: not run.

## casdoor

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2dd5022 | no reflect models (type descriptors merge) | head | 30 | +0 -8 | yes | 3 | 14812 | 60614 | 656 | 12.1 | 2774 | unmodeled_library_call 78, dynamic_dispatch 40, depth_limit 23 |
| 43ba517 | reflect models | head | 30 | +0 -8 | yes | 3 | 13735 | 51634 | 383 | 12.1 | 2827 | unmodeled_library_call 78, dynamic_dispatch 40, depth_limit 23 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 30 | +0 -8 | yes | 3 | 14361 | 44353 | 282 | 19.1 | 2644 | unmodeled_library_call 78, dynamic_dispatch 40, depth_limit 22 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 30 | +0 -8 | yes | 3 | 14753 | 58764 | 596 | 14.1 | 2686 | unmodeled_library_call 78, dynamic_dispatch 40, reflection 19 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 30 | +0 -8 | yes | 3 | 14361 | 44353 | 282 | 21.1 | 2564 | unmodeled_library_call 78, dynamic_dispatch 40, depth_limit 22 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 38 | +0 -0 | yes | 3 | 14633 | 53042 | 282 | 26.2 | 2865 | unmodeled_library_call 128, dynamic_dispatch 40, depth_limit 23 |
| 7552eb9 | writer model puts the data on the whole object | head | 38 | +0 -0 | yes | 3 | 14603 | 53022 | 282 | 37.3 | 2601 | unmodeled_library_call 128, dynamic_dispatch 40, depth_limit 23 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 59 | +29 -8 | yes | 3 | 14814 | 66491 | 538 | 12.1 | 2760 | unmodeled_library_call 78, dynamic_dispatch 40, reflection 19 |
| a1e2fb7 | writer content step (on main) | head | 38 | +0 -0 | yes | 3 | 14633 | 53042 | 282 | 10.1 | 2774 | unmodeled_library_call 128, dynamic_dispatch 40, depth_limit 23 |
| c9aa412 | only the seeds are not written back | head | 30 | +0 -8 | yes | 3 | 14812 | 60614 | 656 | 14.1 | 2737 | unmodeled_library_call 78, dynamic_dispatch 40, reflection 19 |
| de627ce | main (the engine the differences are against) | head | 38 | ref | yes | 3 | 14728 | 53043 | 282 | 10.1 | 2787 | unmodeled_library_call 128, dynamic_dispatch 40, depth_limit 23 |

## gin-vue-admin

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2dd5022 | no reflect models (type descriptors merge) | head | 29 | +0 -0 | yes | 4 | 27819 | 238421 | 2642 | 335.0 | 3861 | dynamic_dispatch 137, reflection 62, unmodeled_library_call 53 |
| 43ba517 | reflect models | head | 29 | +0 -0 | yes | 4 | 27554 | 212399 | 2205 | 330.9 | 3131 | dynamic_dispatch 137, unmodeled_library_call 53, reflection 49 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 29 | +0 -0 | yes | 4 | 25739 | 198449 | 2201 | 26.2 | 5386 | dynamic_dispatch 137, unmodeled_library_call 53, reflection 49 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 29 | +2 -2 | yes | 4 | 27666 | 86815 | 392 | 61.4 | 4568 | dynamic_dispatch 137, reflection 62, unmodeled_library_call 53 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 29 | +0 -0 | yes | 4 | 25739 | 198449 | 2201 | 31.2 | 5346 | dynamic_dispatch 137, unmodeled_library_call 53, reflection 49 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 29 | +0 -0 | yes | 4 | 26362 | 204678 | 2201 | 221.2 | 3814 | dynamic_dispatch 137, unmodeled_library_call 112, reflection 49 |
| 7552eb9 | writer model puts the data on the whole object | head | 29 | +0 -0 | yes | 4 | 26362 | 204721 | 2201 | 265.5 | 4416 | dynamic_dispatch 137, unmodeled_library_call 112, reflection 49 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 29 | +3 -3 | yes | 4 | 27813 | 256224 | 4044 | 36.3 | 5391 | dynamic_dispatch 137, reflection 62, unmodeled_library_call 53 |
| a1e2fb7 | writer content step (on main) | head | 29 | +0 -0 | yes | 4 | 26362 | 204678 | 2201 | 40.3 | 5120 | dynamic_dispatch 137, unmodeled_library_call 112, reflection 49 |
| c9aa412 | only the seeds are not written back | head | 29 | +3 -3 | yes | 4 | 27819 | 238421 | 2642 | 26.2 | 5372 | dynamic_dispatch 137, reflection 62, unmodeled_library_call 53 |
| de627ce | main (the engine the differences are against) | head | 29 | ref | yes | 4 | 30005 | 204678 | 2201 | 37.3 | 5304 | dynamic_dispatch 137, unmodeled_library_call 112, reflection 49 |

## gitea-cut

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2dd5022 | no reflect models (type descriptors merge) | head | 12 | +0 -0 | yes | 1 | 1410 | 4697 | 307 | 3.0 | 447 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 26 |
| 43ba517 | reflect models | head | 12 | +0 -0 | yes | 1 | 1410 | 4697 | 307 | 3.0 | 498 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 26 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 12 | +0 -0 | yes | 2 | 1423 | 4722 | 307 | 4.0 | 400 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 27 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 12 | +0 -0 | yes | 1 | 1410 | 4056 | 294 | 2.0 | 535 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 18 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 12 | +0 -0 | yes | 2 | 1423 | 4722 | 307 | 4.0 | 468 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 27 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 12 | +0 -0 | yes | 2 | 1423 | 4764 | 307 | 5.0 | 508 | unmodeled_library_call 5861, dynamic_dispatch 478, depth_limit 28 |
| 7552eb9 | writer model puts the data on the whole object | head | 12 | +0 -0 | yes | 2 | 1423 | 4764 | 307 | 4.0 | 497 | unmodeled_library_call 5861, dynamic_dispatch 478, depth_limit 28 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 12 | +0 -0 | yes | 1 | 1410 | 6620 | 352 | 2.0 | 492 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 25 |
| a1e2fb7 | writer content step (on main) | head | 12 | +0 -0 | yes | 2 | 1423 | 4764 | 307 | 3.0 | 488 | unmodeled_library_call 5861, dynamic_dispatch 478, depth_limit 28 |
| c9aa412 | only the seeds are not written back | head | 12 | +0 -0 | yes | 1 | 1410 | 4697 | 307 | 7.0 | 457 | unmodeled_library_call 5857, dynamic_dispatch 471, depth_limit 22 |
| de627ce | main (the engine the differences are against) | head | 12 | ref | yes | 2 | 1423 | 4764 | 307 | 3.0 | 508 | unmodeled_library_call 5861, dynamic_dispatch 478, depth_limit 28 |

## go-scale

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 10fabef | no absorbWhole (join + widen) | head | 136 | +82 -0 | yes | 11 | 6774 | 72932 | 85 | 6.1 | 353 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 2dd5022 | no reflect models (type descriptors merge) | head | 218 | +164 -0 | NO | 12 | 8187 | 76052 | 164 | 6.0 | 382 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 43ba517 | reflect models | head | 136 | +82 -0 | yes | 11 | 6774 | 72932 | 85 | 4.0 | 391 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 54 | +0 -0 | yes | 11 | 4833 | 12003 | 37 | 3.0 | 359 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 218 | +164 -0 | NO | 12 | 8187 | 76052 | 164 | 4.0 | 432 | dynamic_dispatch 93, depth_limit 53, unmodeled_library_call 30 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 54 | +0 -0 | yes | 11 | 4833 | 12003 | 37 | 2.0 | 349 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 54 | +0 -0 | yes | 11 | 4833 | 12003 | 37 | 4.0 | 363 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 7552eb9 | writer model puts the data on the whole object | head | 54 | +0 -0 | yes | 11 | 4833 | 12003 | 37 | 3.0 | 366 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 218 | +164 -0 | NO | 12 | 8187 | 76052 | 164 | 3.0 | 384 | dynamic_dispatch 93, depth_limit 53, unmodeled_library_call 30 |
| a1e2fb7 | writer content step (on main) | head | 54 | +0 -0 | yes | 11 | 4833 | 12003 | 37 | 3.0 | 384 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| aa7cb0c | absorbWhole | head | 136 | +82 -0 | yes | 11 | 6774 | 72812 | 85 | 4.0 | 367 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |
| c9aa412 | only the seeds are not written back | head | 218 | +164 -0 | NO | 12 | 8187 | 76052 | 164 | 4.0 | 393 | dynamic_dispatch 93, depth_limit 53, unmodeled_library_call 30 |
| de627ce | main (the engine the differences are against) | head | 54 | ref | yes | 11 | 4833 | 12003 | 37 | 3.0 | 416 | dynamic_dispatch 93, depth_limit 62, unmodeled_library_call 30 |

## go-synth-S

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 10fabef | no absorbWhole (join + widen) | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 146 | dynamic_dispatch 17, depth_limit 6, unmodeled_library_call 5 |
| 2dd5022 | no reflect models (type descriptors merge) | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 143 | dynamic_dispatch 17, depth_limit 6, unmodeled_library_call 5 |
| 43ba517 | reflect models | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 150 | dynamic_dispatch 17, depth_limit 6, unmodeled_library_call 5 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 25 | +0 -0 | yes | 1 | 216 | 2184 | 168 | 2.0 | 144 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 153 | dynamic_dispatch 17, unmodeled_library_call 5, depth_limit 3 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 25 | +0 -0 | yes | 1 | 216 | 2184 | 168 | 2.0 | 144 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 25 | +0 -0 | yes | 1 | 216 | 2199 | 168 | 3.0 | 132 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| 7552eb9 | writer model puts the data on the whole object | head | 25 | +0 -0 | yes | 1 | 216 | 2199 | 168 | 4.0 | 148 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 136 | dynamic_dispatch 17, unmodeled_library_call 5, depth_limit 3 |
| a1e2fb7 | writer content step (on main) | head | 25 | +0 -0 | yes | 1 | 216 | 2199 | 168 | 2.0 | 138 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| aa7cb0c | absorbWhole | head | 25 | +0 -0 | yes | 1 | 216 | 2184 | 168 | 3.0 | 142 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |
| c9aa412 | only the seeds are not written back | head | 25 | +0 -0 | yes | 1 | 214 | 2461 | 168 | 3.0 | 141 | dynamic_dispatch 17, unmodeled_library_call 5, depth_limit 3 |
| de627ce | main (the engine the differences are against) | head | 25 | ref | yes | 1 | 216 | 2199 | 168 | 2.0 | 137 | dynamic_dispatch 17, depth_limit 5, unmodeled_library_call 5 |

## go-test-bench

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2dd5022 | no reflect models (type descriptors merge) | head | 37 | +2 -2 | yes | 3 | 6526 | 24502 | 228 | 12.1 | 1092 | unmodeled_library_call 110, dynamic_dispatch 105, depth_limit 26 |
| 43ba517 | reflect models | head | 37 | +2 -2 | yes | 3 | 6561 | 23573 | 228 | 11.1 | 953 | unmodeled_library_call 110, dynamic_dispatch 105, depth_limit 26 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 37 | +2 -2 | yes | 2 | 6462 | 17515 | 218 | 6.0 | 1152 | unmodeled_library_call 110, dynamic_dispatch 105, depth_limit 26 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 37 | +2 -2 | yes | 2 | 6480 | 24395 | 228 | 6.0 | 1177 | unmodeled_library_call 110, dynamic_dispatch 105, unresolved_callee 6 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 37 | +2 -2 | yes | 2 | 6462 | 17515 | 218 | 5.0 | 1074 | unmodeled_library_call 110, dynamic_dispatch 105, depth_limit 26 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 37 | +0 -0 | yes | 2 | 6375 | 19007 | 218 | 9.1 | 1030 | unmodeled_library_call 135, dynamic_dispatch 106, depth_limit 26 |
| 7552eb9 | writer model puts the data on the whole object | head | 37 | +0 -0 | yes | 2 | 6375 | 19004 | 218 | 9.1 | 1067 | unmodeled_library_call 135, dynamic_dispatch 106, depth_limit 26 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 37 | +3 -3 | yes | 3 | 6526 | 24786 | 228 | 6.1 | 1039 | unmodeled_library_call 110, dynamic_dispatch 105, depth_limit 8 |
| a1e2fb7 | writer content step (on main) | head | 37 | +0 -0 | yes | 2 | 6375 | 19007 | 218 | 6.0 | 1042 | unmodeled_library_call 135, dynamic_dispatch 106, depth_limit 26 |
| c9aa412 | only the seeds are not written back | head | 37 | +2 -2 | yes | 3 | 6526 | 24502 | 228 | 5.0 | 1034 | unmodeled_library_call 110, dynamic_dispatch 105, unresolved_callee 6 |
| de627ce | main (the engine the differences are against) | head | 37 | ref | yes | 2 | 6406 | 19007 | 218 | 8.0 | 1125 | unmodeled_library_call 135, dynamic_dispatch 106, depth_limit 26 |

## gogs-48946299

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 10fabef | no absorbWhole (join + widen) | head | 78 | +1 -3 | yes | 4 | 22475 | 67395 | 410 | 39.2 | 3386 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 68 |
| 2dd5022 | no reflect models (type descriptors merge) | head | 99 | +22 -3 | yes | 5 | 28251 | 361245 | 1413 | 209.3 | 2884 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 141 |
| 43ba517 | reflect models | head | 78 | +1 -3 | yes | 4 | 22475 | 67395 | 410 | 125.9 | 2920 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 68 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 79 | +0 -1 | yes | 4 | 23324 | 52482 | 314 | 13.1 | 3458 | dynamic_dispatch 704, unmodeled_library_call 174, assumed_call 153 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 69 | +2 -13 | yes | 5 | 25141 | 92397 | 435 | 37.2 | 3272 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 143 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 124 | +45 -1 | yes | 5 | 23418 | 53523 | 314 | 13.1 | 3445 | dynamic_dispatch 704, unmodeled_library_call 174, assumed_call 153 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 80 | +0 -0 | yes | 4 | 23543 | 59880 | 314 | 45.2 | 3079 | dynamic_dispatch 729, unmodeled_library_call 295, assumed_call 157 |
| 7552eb9 | writer model puts the data on the whole object | head | 80 | +0 -0 | yes | 4 | 23543 | 59812 | 314 | 60.3 | 3142 | dynamic_dispatch 729, unmodeled_library_call 295, assumed_call 157 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 99 | +23 -3 | yes | 5 | 28255 | 471156 | 1919 | 31.2 | 3828 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 120 |
| a1e2fb7 | writer content step (on main) | head | 80 | +0 -0 | yes | 4 | 23543 | 59880 | 314 | 20.1 | 3515 | dynamic_dispatch 729, unmodeled_library_call 295, assumed_call 157 |
| aa7cb0c | absorbWhole | head | 78 | +1 -3 | yes | 4 | 21972 | 52566 | 314 | 21.2 | 3468 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 69 |
| c9aa412 | only the seeds are not written back | head | 99 | +22 -3 | yes | 5 | 28251 | 361245 | 1413 | 26.2 | 3818 | dynamic_dispatch 704, unmodeled_library_call 174, depth_limit 121 |
| de627ce | main (the engine the differences are against) | head | 80 | ref | yes | 4 | 23919 | 59870 | 314 | 20.1 | 3879 | dynamic_dispatch 729, unmodeled_library_call 295, assumed_call 157 |

## gogs-9a9388ac

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 10fabef | no absorbWhole (join + widen) | head | 78 | +1 -3 | yes | 4 | 17719 | 59994 | 410 | 38.2 | 3339 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 60 |
| 2dd5022 | no reflect models (type descriptors merge) | head | 78 | +1 -3 | yes | 4 | 19055 | 65718 | 392 | 82.5 | 2698 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 62 |
| 43ba517 | reflect models | head | 78 | +1 -3 | yes | 4 | 17719 | 59994 | 410 | 68.4 | 2943 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 60 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 79 | +0 -1 | yes | 4 | 18477 | 47650 | 314 | 12.1 | 3190 | dynamic_dispatch 703, unmodeled_library_call 172, assumed_call 152 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 67 | +0 -13 | yes | 4 | 17793 | 62261 | 392 | 32.2 | 3277 | dynamic_dispatch 703, unmodeled_library_call 172, numeric_stop 22 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 124 | +45 -1 | yes | 5 | 18570 | 48691 | 314 | 12.1 | 3175 | dynamic_dispatch 703, unmodeled_library_call 172, assumed_call 152 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 80 | +0 -0 | yes | 4 | 18724 | 54935 | 314 | 49.3 | 2749 | dynamic_dispatch 728, unmodeled_library_call 291, assumed_call 156 |
| 7552eb9 | writer model puts the data on the whole object | head | 80 | +0 -0 | yes | 4 | 18724 | 54788 | 314 | 51.3 | 2707 | dynamic_dispatch 728, unmodeled_library_call 291, assumed_call 156 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 78 | +2 -4 | yes | 4 | 18351 | 73132 | 517 | 12.1 | 3545 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 41 |
| a1e2fb7 | writer content step (on main) | head | 80 | +0 -0 | yes | 4 | 18724 | 54935 | 314 | 28.2 | 3613 | dynamic_dispatch 728, unmodeled_library_call 291, assumed_call 156 |
| aa7cb0c | absorbWhole | head | 78 | +1 -3 | yes | 4 | 17581 | 47795 | 314 | 31.2 | 3181 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 60 |
| c9aa412 | only the seeds are not written back | head | 78 | +1 -3 | yes | 4 | 19055 | 65718 | 392 | 13.1 | 3181 | dynamic_dispatch 703, unmodeled_library_call 172, depth_limit 42 |
| de627ce | main (the engine the differences are against) | head | 80 | ref | yes | 4 | 18944 | 54935 | 314 | 22.1 | 3712 | dynamic_dispatch 728, unmodeled_library_call 291, assumed_call 156 |

## gophish

| engine | what | inputs | findings | vs ref | conv | passes | reanalyses | summary_elems | max summary | wall s | peak MB | uncertainties (top 3) |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2dd5022 | no reflect models (type descriptors merge) | head | 8 | +0 -0 | yes | 3 | 3396 | 40991 | 1017 | 6.1 | 1553 | unmodeled_library_call 76, depth_limit 28, dynamic_dispatch 28 |
| 43ba517 | reflect models | head | 8 | +0 -0 | yes | 3 | 3346 | 41115 | 1023 | 7.1 | 1494 | unmodeled_library_call 76, depth_limit 28, dynamic_dispatch 28 |
| 5dfec7f | closure captures; plan D (identity global stores) | head | 8 | +0 -0 | yes | 2 | 2940 | 30925 | 936 | 7.0 | 1494 | unmodeled_library_call 76, dynamic_dispatch 28, depth_limit 26 |
| 623b620 | explicit global writes dropped (CORE-H) | head | 8 | +1 -1 | yes | 3 | 3183 | 15445 | 408 | 7.0 | 1508 | unmodeled_library_call 76, dynamic_dispatch 28, depth_limit 10 |
| 697ae9d | 5dfec7f without plan D (bench/backtest-patches/8-no-plan-d.patch; the commit itself is in no ref) | head | 8 | +0 -0 | yes | 2 | 2940 | 30925 | 936 | 6.0 | 1560 | unmodeled_library_call 76, dynamic_dispatch 28, depth_limit 26 |
| 6ddee62 | writer content step (a branch commit; the same patch is a1e2fb7 on main) | head | 8 | +0 -0 | yes | 2 | 2940 | 31176 | 936 | 7.1 | 1578 | unmodeled_library_call 92, dynamic_dispatch 32, depth_limit 26 |
| 7552eb9 | writer model puts the data on the whole object | head | 8 | +0 -0 | yes | 2 | 2940 | 31173 | 936 | 7.0 | 1567 | unmodeled_library_call 92, dynamic_dispatch 32, depth_limit 26 |
| 8e8b866 | param source written back (N x M, CORE-A) | head | 8 | +1 -1 | yes | 3 | 3380 | 44287 | 1135 | 7.1 | 1578 | unmodeled_library_call 76, dynamic_dispatch 28, depth_limit 21 |
| a1e2fb7 | writer content step (on main) | head | 8 | +0 -0 | yes | 2 | 2940 | 31176 | 936 | 13.1 | 1489 | unmodeled_library_call 92, dynamic_dispatch 32, depth_limit 26 |
| c9aa412 | only the seeds are not written back | head | 8 | +1 -1 | yes | 3 | 3396 | 40991 | 1017 | 7.0 | 1570 | unmodeled_library_call 76, dynamic_dispatch 28, depth_limit 23 |
| de627ce | main (the engine the differences are against) | head | 8 | ref | yes | 2 | 2941 | 31176 | 936 | 10.1 | 1502 | unmodeled_library_call 92, dynamic_dispatch 32, depth_limit 26 |

