# Exploration log

One section per measurement. "Anomaly" sections are for numbers that look wrong and are not explained yet.

## Anomaly 1: Gitea cut-out with dependencies, 865 s (2026-11-21)

**What was seen.** `bench/gitea-cve-paths.md` section 4 recorded the cut-out with dependencies (7 packages of Gitea v1.26.4, `GOMEMLIMIT=14GiB`) at 865 s (ir 80, graph 31, taint 751), 8.9 GB, `sys` 1565 s, against 122 s for the whole `./...`. A cut-out that is slower than the whole looks wrong.

**Measured again** (one run, nothing else heavy running, 76 % free memory, rules `bench/gitea/rules.yaml` with the `remote_response` source, current main):

| run | real | user | sys | max RSS | findings |
|---|---|---|---|---|---|
| cut-out with dependencies | 198 s | 615 s | 297 s | 11.0 GB | 18 |
| whole `./...` with dependencies | 433 s | 981 s | 1070 s | 11.2 GB | 275 |

Stats of the cut-out run: `dep_spent_ms` 1,394,980 (summed over the workers), `own_spent_ms` 5,729, `duration_ms` 163,794.

**Result: does not reproduce.** The cut-out is faster than the whole (198 s against 433 s), not slower. The 865 s run was probably made on a loaded machine (other analyses at the same time, memory pressure: its `sys` time was 1565 s). The whole run is slower now than the 122 s of `t1264-final` too (433 s here, also with other jobs on the machine and with more work in the engine since), so the absolute numbers move with the load; compare only runs made together.

**Where the time is.** Own code (the 7 packages) takes 5.7 s of the work, the dependencies 1395 s: 99.6 % of the summed time is in functions of `Dep` code, as the cut-out has few own functions and many dependency functions reached from them. Reading the places that use `fn.Dep` (`internal/taint/engine.go`: the ordering of `e.states` puts own code first; `collect` skips the findings and uncertainties of `Dep` functions; the spent time is summed in two buckets): nothing is counted twice or analysed twice there. What is unexplained, and is a question for the next measurement rather than a defect found: why the dependencies of a small own set cost 1395 s of work when the whole run's own set (all of Gitea) reaches the same dependencies. If it recurs, take the per-function spent time of the cut-out's top functions (`-cpuprofile`) and see whether the same dependency function is summarised under several different own contexts.
