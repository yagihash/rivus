# The backtest: does the light bench see what only the big projects showed?

The gate of a change is the light tier of `bench/compare.sh` plus `go test ./...` (about 10 minutes, `bench/README.md`). No big real codebase
is a step of it. This page is the proof that it is enough, and the way to keep proving it: **the bad versions of 2026-11-21 are run on the
bench of today, and the bench must be red on each of them, for the right metric, and green on the good version next to it.**

## How it is run

The inputs are the ones of the head (the fixtures, the answer keys, the rules of the bench, the generator of `go-scale`, the pinned corpora);
only the engine is old.

* **Layers A and curated** (`go test`): `RIVUS_BINARY=<build of cmd/rivus at that version> go test ./cmd/rivus -run 'TestScaleSynth|TestCuratedGo$' -json`,
  read by `python3 bench/backtest.py judge|all|table`. The test is the head's (the answer key, the expected files of `bench/go-scale/expected`),
  so the metric of a red is the one that the gate itself uses.
* **Determinism**: `python3 bench/backtest.py det <binary>`: the writers of one global from the same level (`testdata/go/cases/paramshare/gwriters`),
  60 runs with GOMAXPROCS and `-workers` changed, rules in the oldest format. A different output is red.
* **Layers B and C** (the corpora, the cut-out of the CVEs of layer B): `bench/compare.sh --fixed-inputs --light <good> <bad>`: both engines on the
  inputs of this tree; the table says LOST (a finding of the good build that the bad one does not have), added (NOISE: more than 20 percent and more than 5), the
  sizes and the convergence.
* An old build that cannot read an input (a rules file in a newer format: `parts`, `field`, `calls:`) is **UNJUDGED**: neither red nor green. The
  zoo and the shared `testdata/go/rules.yaml` are in the new format, so they do not judge old builds; the rows below use `go-scale`, `go-curated` and the
  corpora, whose rules are in the old format, and the determinism row has a file of its own.
* "Good" means green on the metric of the row. Other metrics may be red on a good version because a *later* fix is not in it yet (for example the
  curated flows of the closures, the uncertainties by kind): they are not read as the row's, and the table names the metric.

## The table: which bad version is red, in which layer, by which metric

| # | what | bad | good | layer | red by (bad) | good |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | a merge through the descriptor of a type (no `reflect` models) | `2dd5022` | `43ba517` | A: `scale/types`, `scale/typeschain` | types: FAKE 1 (a flow into a quiet sink), `summary_elems` x11.2, `reanalyses` x1.8, `passes` x2.0, largest summary x12.7; typeschain: NOCONV (12 passes), largest summary x24 | green on both |
| 2 | join + widen: the summary grows and shrinks (no `absorbWhole`) | `10fabef` | `aa7cb0c` | A: `scale/widen` | largest summary x6.7 (47 against 7) | green |
| 3 | a logger that goes through `os.Stderr` (no `holds: nothing`) | `671777d` | `2884c01` | A: `scale/stderr` | FAKE 1, `summary_elems` x13.1, `reanalyses` x1.9, largest summary x14.2 | green |
| 4 | 1 to 3 together (the unconverged Gitea, in small) | `2dd5022` | `6c5c28c` | A: `scale/all` (2,500 functions, all shapes) | FAKE 2, NOCONV, `summary_elems` x5.5, largest summary x3.6 | green on FAKE, NOCONV and SIZE (TRACE only: the uncertainties of later fixes) |
| 5 | the writer of a global differs from run to run | `009c0d5` | `396d477` | determinism (`backtest.py det`) | 3 different outputs in 60 runs (10 of 60 differ from the most common) | the same output in 60 runs |
| 6 | the write to an explicit global is dropped (CORE-H) | `623b620` | `c9aa412` | C (compare, fixed inputs) | LOST 11 (gogs, first copy), LOST 31 (gogs, second copy), LOST 3 (gin-vue-admin), LOST 1 (gophish) | (the base of the pair) |
| 7 | the source of a param is written back: N x M (CORE-A) | `8e8b866` | `c9aa412` | C (compare, fixed inputs) | casdoor +29 findings (97 percent: NOISE), gogs (second copy) LOST 1 | (the base) |
| 8 | save and restore lifts a sub-cell to the root (the noise of C-7) | `697ae9d` (`5dfec7f` without plan D; made for this, not pushed) | `5dfec7f` | curated (`go-curated/gitea`) | FAKE 1 more than the good version | see the ledger |
| 9 | regressions of the traces | optional | | | not run | |
| 10 | a writer model puts the data on the whole object (only +20 on Gitea; the light layer C had +0) | `7552eb9` | `a1e2fb7` (on main; the same patch as the branch commit `6ddee62`) | curated decoys (`bench/go-curated/api/rotate.go`, the writers of a rotating log: `Write(data)`, then the body gives its own file-name field to a sink) | FAKE 2 | green (`TestCuratedGo$` with `RIVUS_BINARY` = a build of `a1e2fb7`); red on `7552eb9` |
| 11 | a path of another entry is folded into the shortest one (the third CVE) | head | | B | recorded as a miss, not a condition | |

Rows 6 and 7 are told apart by the metric: `623b620` is red for 6 (LOST) and was green for 7 by design (its fix for 7 is in it); `8e8b866` is red for 7 (NOISE)
and it has the lost flows of 6 only as a second effect. Rows 1, 3 and 4 are the ones that only a big project showed on 2026-11-21 (the types cache with the chain,
`os.Stderr`, the writer noise) or that overlapped; they are now red in seconds (`TestScaleSynth`, 2 to 14 seconds a profile). Row 10 is the one that the light
corpora did not show (+0 added), and the decoy is the only layer that does.

What the bench still does not do: the unconverged state of Gitea itself (a chain of 12 steps of global writes through type caches) is reproduced only
as `scale/typeschain` (NOCONV at 14 stages) and `scale/all` (NOCONV with the three causes), not as the same numbers. The memos swelling (985,000 elements) is
not explained, and is in `bench/explore/pool.yaml`, not in the gate.
