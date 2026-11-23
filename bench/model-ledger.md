## Rows of the models that the differential test judged (2026-11-23)

`bench/model-diff` counts, per package, the rows of the models that it really checked ("judged") and the ones that it could not ("not judged", with the reason; a row that it cannot judge is never counted as agreeing). A row is judged when a probe showed something about it: a flows row, when one of the two runs (the bodies read, the models) had a flow at a position; a `pure` row, when the body was read at a place that it could write into; a sink row, when its declaration was compared; a source row, when the symbol, the field or the type and the positions that it names are in the package (this is a lookup, not a check of the data). Made by `go run ./bench/model-diff -run-changed 4b825dc642cb6eb9a060e54bf8d69288fbee4904 -out <dir>` (the empty tree as the base: every Go package that has rows) and `go run ./bench/model-diff -dead-allow <dir>`, on the models of main at `6bee840` plus the checks of this change. Go only: Dart (flows and sinks) was not run here, Rust is not checked at all.

| group | packages | rows | judged | not judged | MISS | EXTRA |
|---|---|---|---|---|---|---|
| Go standard library (go-stdlib, go-stdlib-more, go-pure, go-net-http, go-config) | 77 | 617 | 342 (55%) | 275 | 39 | 0 |
| go-github (`github.com/google/go-github/v*/github`, v93.0.0) | 1 | 79 | 76 (96%) | 3 | 0 | 0 |
| gin, sqlx, grpc, Google Cloud (spanner, storage, bigquery) | 7 | 144 | 117 | 27 | 9 | 0 |

The 39 MISS of the standard library are all `pure` rows (14 packages: `crypto/*`, `log/slog`, `net`, `net/http`, `net/smtp`, `os`, `path/filepath`, `go/printer`), and are the first time that a `pure` row was read against its body; the 9 MISS of spanner are flows rows (`Row.Columns`, `ColumnByName`, `ToStruct`, `ToStructLenient`) and are the same on main before this change. Not judged in the standard library: the source cannot be put into the receiver 98, no position that the source can be put into 50, method of an interface 40, no flow in either run 39, a type that the source cannot be put into 32, other 16. go-github: 49 of 52 sink rows and 27 of 27 source rows; the 3 others take a callback.

The rows that the zoo does not hit (`internal/model/builtin/*.dead-allow`), and whether a differential test confirmed them (judged, and no MISS or EXTRA on the row):

| language | `.dead-allow` rows | confirmed | judged, but the row is wrong (MISS) | not judged | no differential test |
|---|---|---|---|---|---|
| Go | 594 | 330 (56%) | 31 | 233 | 0 |
| Dart | 451 | 0 | 0 | 0 | 451 (not run for this count) |
| Rust | 428 | 0 | 0 | 0 | 428 (not checked: needs the isolation) |
