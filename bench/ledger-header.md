**Unverified rows: 0** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main). It is counted again at every change of this file.

No row is unverified. Row 10 (a writer model puts the data on the whole object) was the last one: its good version (the writer content step, `a1e2fb7`, on main; the branch commit `6ddee62` has the same patch) is green on `bench/go-curated/api/rotate.go`, and its bad version (`7552eb9`) is red on it (`TestCuratedGo$` of the head, with `RIVUS_BINARY` set to a build of each: FAKE 2 on the bad one, green on the good one).

Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.

**The day the number reached 0: 2026-11-21** (the commit that renamed `rotate.go.pending` to `rotate.go`; a row that is added later starts as unverified).

Holes: only the pairs of rows 6, 7, 8 (partly) and 10 have layer C runs so far; the rows of A (go-scale) are in BACKTEST.md. Wall times were taken while the machine was shared (read them as a guide; the counts are exact).
