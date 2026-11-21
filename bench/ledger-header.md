**Unverified rows: 1** (the number of rows of `bench/BACKTEST.md` that are not red on the bad version AND green on the good version, on main). It is counted again at every change of this file.

| row | why it is unverified | what makes it verified |
|---|---|---|
| 10 (a writer model puts the data on the whole object) | the good version (the writer content step, `6ddee62`) is a branch, not on main. The decoys are in `bench/go-curated/api/rotate.go.pending` (red on `7552eb9`, green on `6ddee62`), and are not a .go file so that the gate stays green until the fix is on main | the fix is on main and the file is renamed to `rotate.go` in the same change |

Rows 9 (the traces; optional) and 11 (a recorded miss, not a condition) are not counted.

**The day the number reached 0: not yet** (write the date and the commit here when it does; a row that is added later starts as unverified).

Holes: only the pairs of rows 6, 7, 8 (partly) and 10 have layer C runs so far; the rows of A (go-scale) are in BACKTEST.md. Wall times were taken while the machine was shared (read them as a guide; the counts are exact).
