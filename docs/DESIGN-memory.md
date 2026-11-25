# Memory: where it goes, and the designs to bring it down

> **Status: 一部実装 (implemented in part); the ADRs and ARCHITECTURE are the reference** (the designs that were here are removed)

On Go with the dependencies read (the default; `-no-deps` leaves them out), **the peak is in loading and in building go/ssa**, not in the analysis. Numbers are not written here: the ledger (`bench/heavy-ledger.tsv`) holds them.

| Phase | What is held |
|---|---|
| load | `packages.Load` with `NeedDeps`: syntax and types of every dependency (about 1 GB) |
| build | go/ssa twice: the naive form (the IR is made from it) and the lifted form (only for the call graph); both of every dependency package |
| convert | the IR of the functions reachable from the project |
| analysis | the graph `sg`, the summaries |

Already done: dependency syntax and type info are let go package by package after the build; the
loaded packages are let go after the IR; only the reachable functions are converted.

## What was decided

The designs of this document (a call graph before the naive build, a soft memory limit, a reachability pass, streaming the IR, export data) are not kept here. What was decided is in the ADRs: the memory limit `-max-memory` (ADR-121) and the IR written to disk per function (ADR-123). The current form is in `docs/ARCHITECTURE.md` (section 16).
