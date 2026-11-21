**Unverified rows: 0** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main; the status of each row is in `bench/backtest-rows.json`). It is counted again at every change of that file.

No row is unverified.

Row 8: the first table of this page named `scale/selfcopy` and `scale/all` for it, and a good version "see the ledger"; the ledger has no such section and both units are green on the bad build. It is judged on `curated-go` (the decoy `api/restore.go`), and the bad build is rebuilt from `5dfec7f` and `bench/backtest-patches/8-no-plan-d.patch`.
Row 10: the good version was a branch commit (`6ddee62`); the same patch is `a1e2fb7` on main, and the decoys were a `.pending` file until the file was enabled.

Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.

**The day the number reached 0: 2026-11-21** (the day the last of the rows was verified; a row that is added later starts as unverified).

Holes: only the pairs of rows 6, 7 (layer C) have compare runs so far, and row 8 and 10 are judged by `TestCuratedGo`; the rows of A (go-scale) are in BACKTEST.md. Wall times were taken while the machine was shared (read them as a guide; the counts are exact).
