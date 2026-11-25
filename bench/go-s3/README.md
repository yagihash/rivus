# go-s3

A small synthetic HTTP server that calls `service/s3` of the AWS SDK for Go v2 (`GetObject`, `PutObject`,
`ListObjectsV2`) with a bucket-side key and prefix taken from the query of the request. About 320 packages, direct
and indirect. It is never run: it is only analyzed (`go build` and the module download are the only things done to
the dependencies). Nothing connects to AWS.

It is the bench for "does `rivus analyze` finish on a product with heavy dependencies": by default (the
dependencies read) the result (finished or not, the time) is the newest `deps` row of the ledger. Run it with `make bench-deps BENCHES=go-s3`; the rows are in
`bench/heavy-ledger.tsv` and `bench/LEDGER.md`. The measurements of the stages of declaring the packages are in
`docs/design-package-models/03-measurements.md` (3.1).

The versions are pinned in `go.mod` / `go.sum`; every module is older than two weeks at the time it was pinned
(2026-11-23).
