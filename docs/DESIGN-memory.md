# Memory: where it goes, and the designs to bring it down

Measured on Go with the dependencies read (the default; `-no-deps` leaves them out): Gogs peaks at 3.3 GB, gin-vue-admin at 5.5 GB; after the
IR is made the live heap is 1.8 GB. **The peak is in loading and in building go/ssa**, not in the
analysis.

| Phase | What is held |
|---|---|
| load | `packages.Load` with `NeedDeps`: syntax and types of every dependency (about 1 GB) |
| build | go/ssa twice: the naive form (the IR is made from it) and the lifted form (only for the call graph); both of every dependency package |
| convert | the IR of the functions reachable from the project |
| analysis | the graph `sg`, the summaries |

Already done: dependency syntax and type info are let go package by package after the build; the
loaded packages are let go after the IR; only the reachable functions are converted.

## Designs, in the order I would do them

**A. Call graph first, then the naive build, one package at a time (expected: peak about 60%).**
Build the lifted program, make the call graph (VTA), keep from it only what the IR needs (for each
call site: the candidates, keyed by function and call index), drop the lifted program, and only then
build the naive form, package by package in the order of their paths, converting what is reachable
and dropping the package's go/ssa at once. Now the two programs are never alive together, and the
naive one is never whole. Costs: a small table of call-site candidates; the conversion must find a
call by function id and index (it finds it by the instruction now).

**B. Soft memory limit (cheap).** `GOMEMLIMIT` at a fraction of the physical memory (`-memory-limit`,
default 70%): the collector works harder near the limit instead of letting garbage pile up (about
20% of the peak is garbage). Costs time, near the limit only. No design risk.

**C. A reachability pass on types before building (larger gain, larger work).** From the syntax and
`types.Info.Uses` of the project, find which packages and functions are referred to (calls by name,
methods by name for interface calls), and build only the packages that can be reached. Most large
dependencies (cloud SDKs) are imported for a few functions. Costs: an approximation of the call graph
made before go/ssa exists; an unreached package that holds an implementation of an interface
could be missed (its methods must count as reached when its type is converted to an interface that
is reached).

**D. Stream the IR into the graph (about 0.5 GB).** Make `sg` while the IR is being converted, not
after. Measured small for Go; only if A to C are not enough.

**E. Load dependencies from export data when they are not reached.** A first load without
`NeedDeps` gives the direct imports; a second, for the packages C says are reached. Needs C first.

Order: B now (a few lines), then A, then C (with E). Each is measured on Gogs and gin-vue-admin
with the same findings (the check is "the output is the same", as it was for the changes above).
