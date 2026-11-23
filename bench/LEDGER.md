**Unverified rows: 0** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main; the status of each row is in `bench/backtest-rows.json`). It is counted again at every change of that file.

No row is unverified.

Row 8: the first table of this page named `scale/selfcopy` and `scale/all` for it, and a good version "see the ledger"; the ledger has no such section and both units are green on the bad build. It is judged on `curated-go` (the decoy `api/restore.go`), and the bad build is rebuilt from `5dfec7f` and `bench/backtest-patches/8-no-plan-d.patch`.
Row 10: the good version was a branch commit (`6ddee62`); the same patch is `a1e2fb7` on main, and the decoys were a `.pending` file until the file was enabled.

Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.

**The day the number reached 0: 2026-11-21** (the day the last of the rows was verified; a row that is added later starts as unverified).

Holes: only the pairs of rows 6, 7 (layer C) have compare runs so far, and row 8 and 10 are judged by `TestCuratedGo`; the rows of A (go-scale) are in BACKTEST.md. Wall times were taken while the machine was shared (read them as a guide; the counts are exact).

## Rows of the models that the differential test judged (2026-11-23)

`bench/model-diff` counts, per package, the rows of the models that it really checked ("judged") and the ones that it could not ("not judged", with the reason; a row that it cannot judge is never counted as agreeing). A row is judged when a probe showed something about it: a flows row, when one of the two runs (the bodies read, the models) had a flow at a position; a `pure` row, when the body was read at a place that it could write into; a sink row, when its declaration was compared; a source row, when the symbol, the field or the type and the positions that it names are in the package (this is a lookup, not a check of the data). Made by `go run ./bench/model-diff -run-changed 4b825dc642cb6eb9a060e54bf8d69288fbee4904 -out <dir>` (the empty tree as the base: every Go package that has rows) and `go run ./bench/model-diff -dead-allow <dir>`, on the models of main at `6bee840` plus the checks of this change. Go only: Dart (flows and sinks) was not run here, Rust is not checked at all.

Before and after the change that read the `pure` rows against their bodies and gave a struct the source in its exported fields (2026-11-23, on the models of `feat/model-diff-flow-rows` and of this change):

| group | packages | rows | judged before | judged after | not judged before | not judged after | MISS before | MISS after | EXTRA |
|---|---|---|---|---|---|---|---|---|---|
| Go standard library (go-stdlib, go-stdlib-more, go-pure, go-net-http, go-config) | 77 | 617 | 342 (55%) | 348 (56%) | 275 (44.6%) | 269 (43.6%) | 39 | 12 | 0 |
| go-github (`github.com/google/go-github/v*/github`, v93.0.0) | 1 | 79 | 76 (96%) | 76 | 3 | 3 | 0 | 0 | 0 |
| gin, sqlx, grpc, Google Cloud (spanner, storage, bigquery) | 7 | 144 | 117 | 117 | 27 | 27 | 9 | 0 | 0 |

The 39 MISS of the standard library before were all `pure` rows (14 packages: `crypto/*`, `log/slog`, `net`, `net/http`, `net/smtp`, `os`, `path/filepath`, `go/printer`), the first time that a `pure` row was read against its body; the 9 MISS of spanner were flows rows (`Row.Columns`, `ColumnByName`, `ToStruct`, `ToStructLenient`). After: the `pure` rows of the real flows are `flows:` rows, the ones that carry no data are `miss-reviewed` (14 positions in the standard library and 4 in spanner, listed in the pull request), spanner has no MISS. The 12 MISS that are left: `ServeMux.Handle` (pattern into the mux, a candidate that is not changed here) and `filepath.Walk` (the branch `fix/model-sanitizer-callbacks`) were there before; the other 10 are new findings on rows that this change did not touch, shown only because a struct can now be given the source (`Client.Do` arg0 to the error, `URL.Port`, `tls.Client` arg1, `x509.CreateCertificate` arg1 and arg2, `x509.DecryptPEMBlock`, `httputil.DumpRequest`, `zip.Writer.CreateHeader`, `Record.AddAttrs`, `Cmd.CombinedOutput`; not triaged). Not judged in the standard library, before and after (verified on both runs; the sum is 275 and 269 rows): the source cannot be put into the receiver 98 and 82, no position that the source can be put into 50 and 50, method of an interface 40 and 40, no flow in either run 39 and 55 (counted as not judged: a probe where neither run has a flow tells nothing about the row, because the same result comes from a row that is right and from one that is wrong; a `pure` row is judged when it has a place to look at, even without a flow; some of the 16 more are rows that were receiver or type reasons before: the struct was now given the source, and only the row has a flow, which shows nothing about the row), a type that the source cannot be put into 32 and 25, other 16 and 17. The target of 40% for the standard library was not reached (43.6%): what is left of the first reason is the receivers that have no exported field that holds text (locks, times, handles of a database or a connection, `reflect.Value`), and the next two (no position, a method of an interface) are rows that have nothing to put the source into or no body to read. go-github: 49 of 52 sink rows and 27 of 27 source rows; the 3 others take a callback.

The rows that the zoo does not hit (`internal/model/builtin/*.dead-allow`), and whether a differential test confirmed them (judged, and no MISS or EXTRA on the row):

| language | `.dead-allow` rows | confirmed | judged, but the row is wrong (MISS) | not judged | no differential test |
|---|---|---|---|---|---|
| Go, before | 594 | 330 (56%) | 31 | 233 | 0 |
| Go, after | 594 | 353 (59%) | 9 | 232 | 0 |
| Dart | 451 | 0 | 0 | 0 | 451 (not run for this count) |
| Rust | 428 | 0 | 0 | 0 | 428 (not checked: needs the isolation) |

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

## Benches with heavy dependencies (`make bench-deps`)

`rivus analyze` with no flag that changes the analysis, on a local run (bench/deps_bench.py; the columns are described there). One row per run; compare the same bench between dates. `finished`: yes / timeout / oom / error. `-` is not measured (max RSS of a run that we killed). The engine is the commit the binary was built from.

| date | rivus | bench | lang | wall_s | max_rss_mb | functions | findings | uncertainties | finished | timeout_s |
|---|---|---|---|---|---|---|---|---|---|---|
| 2026-11-23 | 6bee840 | go-s3 | go | 686.6 | 12449 | 5845 | 1 | 47 | timeout | 600 |
| 2026-11-23 | 6bee840 | gcp | go | 70.7 | 4012 | 20810 | 27 | 143 | yes | 600 |
| 2026-11-23 | 6bee840 | dart-gcp | dart | 44.4 | 1530 | 4593 | 19 | 738 | yes | 600 |
## The pass line (`make bench-passline`)

The goal: rivus analyses a real product with the default flags, in a realistic time and memory, with a meaningful result. One line per target (bench/deps_bench.py; the targets, the pinned commits and the limits are data in bench/deps-targets.json):

`PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> finish=<yes|no> detected=<yes|no|n/a> quiet_after_fix=<yes|no|n/a> located=<n>`

The pass line is two separate conditions, and PASS needs both (decided 2026-11-24). The default flags are used (no `-no-deps`, no `-body-replaced`).

1. **It finishes** (`finish`), for memos (usememos/memos 26.11.2, `00fcd170281a75fc0e95ec4f2f54f43d32fe81bd`): it ends by itself within 180 s; the peak memory of the whole process tree (the same-time sum, sampled every 0.25 s) stays within 8 GB (the watchdog stops a run above the cap); and it converges (`converged`; no function stopped on the clock).
2. **It detects** (`detected`): CVE-2025-65799 (path traversal; public, fixed in 0.25.3) is found at the pre-fix commit `1d7efb158045924ab87f58d567bd79cd92edf6af` by place: at least one `path_traversal` finding has its source and its sink at the labelled places of the truth file [bench/memos/truth-1d7efb15.json](memos/truth-1d7efb15.json) (builtin models; `located=<n>` is the number of such findings, information). The truth file is read from the public fix (one file, `server/router/api/v1/attachment_service.go`): 1 source place (the request of `CreateAttachment`, line 51) and 3 sink places (`os.MkdirAll` 416, `os.Create` 419, `os.WriteFile` 426 in `SaveAttachmentBlob`). The lines are those of the pre-fix commit; the same sinks at the fixed commit are 422, 425 and 432 (the fix adds two checks before them), kept in the same file as `fixed_line`. A finding of the same kind at other places does not count. The two conditions are judged apart: `finish` does not use the detection, and the detection is read only from a run that converged.

`-no-deps` also meets the detection (a measured fact, 2026-11-24: memos at the pre-fix commit, `-no-deps`, 3 `path_traversal` findings at the 3 sink places, 5.7 s, 653 MB), so the detection does not depend on the dependencies; "finishes with the dependencies" is the other condition and is the one that the default flags have to meet.

`quiet_after_fix` (no `path_traversal` finding at the fixed commit `5f57f48673e2054f404b2c5b497a8eaa3690591d`) is a separate indicator and is not part of PASS/FAIL: rivus does not yet treat a fix of the kind "validate, then return early" as removing the flow (the declared checks of #169 do not exist yet). The fixed commit is analysed only when the pre-fix commit is detected; otherwise it is `n/a`. For a target with a truth file of the other kind (`through_tp`, dart-pub), `detected` is per vulnerability as below and `quiet_after_fix` is `n/a`. The Dart line is cocoon `app_dart` (below); dart_services (dart-pad `pkgs/dart_services`, `ad668d137e631beeaef3e263054a45696e8d4499`) is a load target, printed as `INFO`.

In the ledger ([heavy-ledger.tsv](heavy-ledger.tsv); `python3 bench/heavy_ledger.py`) the two are separate: `finish` (time, memory, `conv`) and `det`; `result` is PASS when both are yes.

**The next form of the "after the fix" condition (decided 2026-11-23, changed 2026-11-24 by ADR-122; not in force yet).** The condition above counts findings of the category, which says nothing about whether the finding went away for the right reason. Once the declared checks of issue #169 exist (a rule file declares a check function with `guards:`, and a finding that passes it is stopped, with a record in `suppressions` that names the rule file and the declaration), the detection condition is run with a rule file that declares the check that the fix added, on both commits: before the fix, with that rule, a finding still appears (the check does not exist yet there); after the fix, with that rule, there is no finding, and `suppressions` names the declaration that stopped it. Without the declaration the finding at the fixed commit stays, which is the evidence that the fix is the check. Until that mechanism exists, the line above is the condition, and a result of "no finding after the fix" (`quiet_after_fix`, where the runner has it) is only recorded, not part of PASS or FAIL.

### The two lines of the pass line: A and C (decided 2026-11-24; both not measured yet)

The user decided that the pass line is two lines, A and C, for different reasons (each was heavy for a different reason). The memos line above (8 GB) stays as it is. The earlier decision "Gitea is not used for the pass line" (2026-11-21) is replaced by this one: Gitea is fetched and run as a target of the pass line. Periodic runs and exploration on it are a separate decision. The result of each is in the ledger (`python3 bench/heavy_ledger.py --kind passline --target <name>`); no number is copied here.

| line | target | flags | time | peak of the tree | other conditions |
|---|---|---|---|---|---|
| A | `memos-4gib`: memos `00fcd170` | the default flags and `-max-memory 4GiB` (the default limit of a machine with 16 GB of physical memory, ADR-121: the same conditions as that machine) | 180 s | 4 GiB | converged; detected (the same places as the memos line) |
| C | `gitea`: Gitea `v1.26.4` (`95ba37d9`, `go-gitea/gitea`) | the default flags, with the dependencies, `-no-project-rules` | 600 s (the limit of the earlier runs, 10 minutes; the 180 s of memos has no ground for Gitea) | 8 GB | converged; `detected=<n>/3` is printed but is **not** part of PASS/FAIL for now |

The detection of C is per published vulnerability of [gitea-cve-truth.json](gitea-cve-truth.json), judged by place (the form of `bench/score.py`). C was added for its size, so the detection is not in the verdict; whether it joins is asked of the user after the line passes. The line and the ledger row of C carry the count `n/3` only: no count of findings, no list, no path, no place. `bench/deps-targets.json` has both targets; the runner reads `detect.mode` `score` (the field `vuln` of the truth file groups the entries of one vulnerability; `detect.in_verdict` false keeps the count out of PASS/FAIL). `gitea` is not in the default `TARGETS` of `make bench-passline` (it takes up to 10 minutes): `make bench-passline TARGETS=gitea`. The time limit is kept by the watchdog of the runner; rivus is not given `-timeout` (its default is no limit), so a run that is cut at the limit is `stopped_time` and not a run that did not converge. The fetch (`git clone`, `git worktree`, `go mod download` into `~/.cache/rivus-bench`) never runs a script or `go generate` of the fetched code. Run it with the heavy-work lock, not at the same time as another heavy run.

### The cocoon target

The Dart target of the pass line is flutter/cocoon `app_dart` (`ae9eb1beaf765d644cc85dde38def030c80b4b7d`). Launch: cwd = `app_dart`, `-allow-deps-under <cocoon>/packages`, `dart pub get` only, the Flutter member of the root workspace removed in the copy (all in `bench/deps_bench.py`; data in `bench/deps-targets.json`). The line is the same as for memos (default flags, within 180 s and 8 GB, converged). `detected=n/a`: the project has no public security advisory, so no pre-fix / fixed pair. dart_services stays a load target for the heavy dependency `package:analyzer` (printed as `INFO`, numbers kept; also a target of the depth experiments).

### Detection: the line is per vulnerability, the detail is per position

`detected` counts per vulnerability, as for memos (which labels the source and the sink places; dart-pub labels the call sites a finding passes through): `yes` when at least one finding passes through a labelled position of that vulnerability's truth (builtin models, default flags). The position-level count is printed beside it as information: `through_tp=<n>/<N>`. The Dart target `dart-pub` (dart-lang/pub `5e24336e71a05a96b558d496953ebed223f6e40a`; one public fixed vulnerability, the zip slip of `extractTarGz`, 9 labelled call sites; `bench/dart-pub`) is printed as INFO (its line is not decided). `bench/score.py --through` gives the same position count from a saved result.

### Peak memory: what is measured how

The memory has two records (`bench/deps_bench.py`). (a) the end-of-run peak of the main process (`ru_maxrss` of the child from `wait4`: the largest single process of its subtree; bytes on macOS). (b) the whole PROCESS TREE: the maximum over fine samples (every 0.25 s) of the SAME-TIME SUM of the RSS of all its processes (shared pages can be counted twice); the biggest process of the tree at that moment and when are kept, so that a large (a)/(b) difference can be explained. The verdict (PASS/FAIL) uses (b): the burden the machine really receives. The watchdog (the same samples; it kills the group of the run only, above the cap or the time limit) only stops a run early.

### Where the numbers are

Every run of `make bench-passline` and `make bench-deps` adds one row to [heavy-ledger.tsv](heavy-ledger.tsv) (the columns, the notes: [heavy-ledger.md](heavy-ledger.md); `python3 bench/heavy_ledger.py` prints it with the margins). A number in a PR or an ADR points to a row (date and commit); it is not copied. `NO_LEDGER=1` adds no row (a binary with a measurement patch, or a run that is not used as a number). `BENCH_OUT` (default `.claude/work/bench-out/`, git-ignored) holds the JSON of the results; the ledger holds numbers only.

## Records before 2026-11-23 (not in the ledger)

These rows were measured before the ledger existed (by hand, with the engine of that day; the commit of rivus is not recorded, and the way of measuring differs from the ledger's). They are kept as they are, numbers only. None was moved to the ledger.

### The starting point (numbers only, a local measurement of the engine of that day)

| target | commit | flags | wall | max memory | converged | functions | findings |
|---|---|---|---|---|---|---|---|
| memos | 00fcd170 | default, cap 8 GB | cut by the cap at 67 s | > 8 GB | no | - | - |
| memos | 00fcd170 | `-timeout 50s` (partial) | 99.5 s (CPU 178 s) | 6.75 GB | no | 40.8k | not meaningful (partial) |
| memos | 00fcd170 | `-body-replaced` of the aws SDK (NOT the default flags; informational) | 45 s | 6.08 GB | yes | 34.7k | - |
| dart_services | ad668d13 | default, cap 8 GB | cut by the cap at 189 s | > 8 GB | no | - | - |
| dart_services | ad668d13 | `-timeout 50s` (partial) | 138 s | 7.98 GB | no | 28.1k | 0 |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | default, cap 8 GB | cut by the cap at 49 s and 31 s | > 8 GB | no | - | - |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | `-body-replaced` of the aws SDK (informational) | 30 s and 23 s | 3.7 GB and 3.6 GB | yes | - | 0 and 0 (`path_traversal`; not detected at the pre-fix commit) |

So today's line for memos is `FAIL memos time=67/180 mem=>8/8 converged=no detected=no quiet_after_fix=n/a` (cut by the cap; not detected at the pre-fix commit with the default flags and the builtin models).

### The cocoon line (numbers only; engine of that day)

| target | commit | flags | wall | max memory | CPU | converged | functions | findings |
|---|---|---|---|---|---|---|---|---|
| cocoon app_dart | ae9eb1be | default, 10m | 101 s | 1.78 GB | 128 s CPU | converged (4 passes) | 16.5k functions (14.3k deps) | 14 findings (ssrf 9, resource_name 4, sql_injection 1) |

Today's line for cocoon: `PASS cocoon time=101/180 mem=1.78/8 converged=yes detected=n/a`.

### Peak memory (numbers only)

The rows marked end-of-run were measured with the earlier method (`/usr/bin/time`, the largest single process, = (a) in the naming above). The earlier rows were sampled, on an older engine and a loaded machine.

| commit | setting | peak memory | how measured |
|---|---|---|---|
| memos 00fcd170 (the pass-line target) | `-dep-k 2` | 8.27 GB | sampled, loaded machine (earlier) |
| memos 00fcd170 | `-dep-k 2` | 8.56 GB | sampled, summed RSS of the process group (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.2 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.8 GB | sampled, summed RSS of the process group (earlier) |
| memos 1d7efb15 (the older pre-fix commit of the detection check, NOT the target) | `-dep-k 2` | 5.9 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 2` | 6.35 GB (163 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |
| memos 00fcd170 | `-dep-k 1` | 5.49 GB (575 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |

The 5.9 GB figure belongs to a different commit than the target; do not compare it with the 00fcd170 rows. The two last rows are the first end-of-run peaks of the target (the wall time of the second one was long: a loaded machine, not measured separately).

### The position count of dart-pub

4/9 with `-no-deps` and the builtin models (an earlier local measurement). With the default flags the run did not converge then, so the line was `detected=no` (INFO).
