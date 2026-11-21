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

## Measurement 2: what if the packages outside every `main` closure were not read as own code (2026-11-21)

**Question.** A user reported scan times growing a lot "because of mocks". `./...` reads every package of the module as own code, also the ones that no non-test `main` imports (mock packages, test helpers). Estimate, before implementing anything, what it would change to analyse only the own packages in the import closure of the `main` packages (`go list -deps -test=false` over all `main` packages, intersected with `go list ./...`). This is a change of the analysis scope (results change), not a speed-up: the findings that disappear are listed first. Names (`mock`, `test`) are used only to describe the set, never to select it. Binary: main at `703c3ac`; rules as in `bench/compare.sh` (memos and cloudreve: built-in only).

### 1. What is outside the closure

| target | own pkgs (`./...`) | mains | outside | lines outside / in closure | `func` decls outside | what they are |
|---|---|---|---|---|---|---|
| memos | 31 | 1 | 4 | 678 / 52,947 | 23 | `internal/cron`, `plugin/openai`, `test`, `test/store` |
| Gitea v1.26.4 | 369 | 3 | 9 | 1,759 / 318,931 | 50 | `tests`, `tests/integration`, `tests/fuzz`, `services/contexttest`, `modules/test`, `modules/indexer/issues/internal/tests`, `models/db/paginator`, `tools/codeformat`, ... |
| gogs (8a3b8198a) | 60 | 2 | 2 | 183 / 46,331 | 3 | `internal/dbtest`, `internal/mocks` |
| gogs (ca59e76a5) | 59 | 2 | 2 | 175 / 45,748 | - | same |
| gin-vue-admin | 47 | 1 | 3 | 197 / 23,221 | 12 | `plugin/ws`, `utils/captcha`, `plugin/plugin-tool/utils` |
| cloudreve | 105 | 1 | 4 | 425 / 157,367 | 37 | `ent/enttest`, `pkg/request/ssrftest`, `pkg/balancer`, `application/migrator/model/dialects` |
| navidrome (not run) | 65 | 1 | 5 | 2,292 / 35,601 | - | `*test`, `tests`, `scanner/metadata_old` |
| casdoor / filebrowser / gophish (not run) | 16 / 19 / 15 | 1 | 2 / 0 / 0 | 231 / 0 / 0 | - | - |

The code outside the closure is 0.4 % to 1.5 % of the lines (navidrome 6 %) in every target, and none of them has a mock package of any size there: the only `mock` named package (gogs `internal/mocks`) is 24 lines. Name hints inside the outside sets: `test` in 2 of 4 (memos), 7 of 9 (Gitea), 1 of 2 (gogs, plus `mock`); the rest are ordinary packages that no binary imports (`plugin/openai`, `utils/captcha`, `pkg/balancer`).

Mock code that exists is **inside** closure packages, which a package-level scope cannot remove: gogs has go-mockgen output `internal/database/mocks_gen.go` (620 lines, one function object per method, no shared controller), `internal/conf/mocks.go`, `internal/gitutil/mocks.go`. No target has gomock, testify/mock, moq or mockery generated code in non-test files; cloudreve (1 test file) and navidrome (9) use them in `_test.go` only, which are not read.

### 2. Before and after

Before: `./...`; after: the own packages of the closure (explicit import paths). Same rules, same flags, `-timeout 1800s` (Gitea after: see the note).

| target | run | wall | user CPU | max RSS | functions | summary_elems | reanalyses | passes | findings (sink groups) | uncertainties (dynamic / unmodeled / unresolved / depth_limit / assumed / reflection) |
|---|---|---|---|---|---|---|---|---|---|---|
| memos | before | 545 s | 873 s | 9.6 GB | 21,808 | 984,610 | 36,114 | 3 | 7 (7) | 824 / 163 / 146 / 258 / 116 / 0 |
| memos | after | 224 s | 828 s | 10.5 GB | 21,758 | 984,562 | 36,062 | 3 | 7 (7) | 819 / 145 / 145 / 258 / 116 / 0 |
| Gitea | before | 658 s | 1,867 s | 10.8 GB | 54,612 | 951,921 | 110,021 | 7 | 348 (97) | 6,601 / 2,088 / 123 / 1,781 / 674 / 123 |
| Gitea | after | 322 s | 1,864 s | 11.7 GB | 54,333 | 950,633 | 109,617 | 7 | 330 (91) | 6,582 / 2,046 / 120 / 1,781 / 669 / 123 |
| gogs 8a3b8198a | before | 11.8 s | 43 s | 3.3 GB | 12,186 | 52,281 | 18,704 | 4 | 79 (42) | 703 / 172 / 9 / 60 / 152 / 14 |
| gogs 8a3b8198a | after | 10.9 s | 41 s | 3.3 GB | 12,168 | 52,272 | 18,686 | 4 | 79 (42) | 701 / 161 / 8 / 55 / 147 / 14 |
| gin-vue-admin | before | 20.3 s | 72 s | 5.2 GB | 19,261 | 203,806 | 25,637 | 4 | 29 (29) | 137 / 53 / 9 / 11 / 18 / 49 |
| gin-vue-admin | after | 20.5 s | 75 s | 5.3 GB | 19,187 | 203,759 | 25,556 | 4 | 29 (29) | 131 / 51 / 9 / 11 / 18 / 49 |
| cloudreve | before | 72 s | 348 s | 8.0 GB | 44,396 | 207,775 | 60,922 | 3 | 25 (25) | 4,230 / 704 / 96 / 784 / 751 / 557 |
| cloudreve | after | 73 s | 358 s | 7.9 GB | 44,302 | 207,687 | 60,828 | 3 | 25 (25) | 4,213 / 690 / 95 / 784 / 749 / 549 |

**Read the wall time with care.** The machine was shared (load average about 20, other analyses running): the same Gitea `before` binary and input gave 658 s here, and the first Gitea `after` run, made while another job was running, took 1,951 s, hit `-timeout 1800s` and did not converge (user CPU 1,277 s, system CPU 4,660 s: memory pressure). That run is discarded (a clock decided it); the table has the rerun with `-timeout 5400s` on a quiet machine. The user CPU time is the comparable number: it is the same before and after in every row (memos 873 / 828, Gitea 1,867 / 1,864). Memos' 545 s against 224 s is load, not the scope (the 120 s of the earlier memos run was on a quiet machine; before and after differ by 0.2 % in functions and 0 in passes).

**Result: the scope change saves nothing measurable on these targets**: 0.1 to 0.5 % of the functions, the same passes and summary elements, the same user CPU; the uncertainties that move are a few dozen entries that sat in the dropped code. Memos still does not get under 120 s: its time is in dependencies (`dep_spent_ms` 3.78 M of 3.80 M; the largest summary is `aws/smithy-go/middleware.BuildStep.HandleMiddleware`), not in its 4 outside packages (678 lines).

### 3. The findings that disappeared

| target | removed | classification |
|---|---|---|
| memos, gogs, gin-vue-admin, cloudreve | 0 sink groups, 0 findings | identical finding sets (sink file, line, category); no group changed its count |
| Gitea | 18 findings in 6 sink groups, all `sql_injection` at `tests/test_utils.go` lines 54, 72, 79, 101, 109, 121 (function `tests.InitTest`) | all six sinks are in the package `code.gitea.io/gitea/tests` (the integration test harness, imported by no `main`); the three sources of each group are production handlers (`routers/install.SubmitInstall`, `httplib.getRequestScheme`, ...) that reach the sink through the harness' init path. **No finding with its sink in production code was lost**, none was added, and no kept sink group changed its count. |

Whether findings in test-harness code are wanted (a SQL string built from install-form values in a harness is a data flow, only not a runtime one) is a product question, not a measurement.

### 4. Mechanism hypothesis: do all mocks meet in `Controller.Call`?

Not confirmed, and unlikely to be the cause:

- **The real targets cannot test it**: none has gomock / testify / moq / mockery code outside `_test.go` (section 1). The only generated mocks are gogs' go-mockgen ones (one function object per method, no shared controller).
- **Synthetic experiment** (N interfaces x M methods, real implementations and consumers in `svc`, a `net/http` handler as the source and `os.Open` as the sink in each implementation, mocks in a package no `main` imports, in gomock shape: `m.ctrl.Call(m, "Name", args...)` and `ret[0].(T)`; also with the real `go.uber.org/mock@v0.6.0` `Controller`, `T.Helper()` and the recorder methods). `./...` against the closure only:

  | shape | functions before / after | reanalyses before / after | summary_elems before / after | findings | dynamic_dispatch sites before / after |
  |---|---|---|---|---|---|
  | 30 x 10, own `Controller` | 1,059 / 694 | 1,059 / 694 | 1,864 / 600 | 30 / 30 | 60 / 60 |
  | 100 x 20, own `Controller` | 6,509 / 4,304 | 6,509 / 4,304 | 12,204 / 4,000 | 100 / 100 | 200 / 200 |
  | 100 x 20, own `Controller`, a helper in the mocks package calls every `UseK(MockIk)` | 6,510 / 4,304 | 6,510 / 4,304 | 12,404 / 4,000 | 100 / 100 | 200 / 200 (every site now has 2 candidates, the mock included) |
  | 30 x 10, real gomock | 1,785 / 694 | 1,792 / 694 | 4,490 / 600 | 30 / 30 | 720 / 60 |
  | 100 x 20, real gomock + the helper | 10,846 / 4,304 | 10,853 / 4,304 | 29,050 / 4,000 | 100 / 100 | 4,400 / 200 |

  Reanalyses equal the number of functions (one pass): **`Controller.Call` did not become a confluence point** (it does not store the arguments, so nothing flows from one mock's call to another's). Mocks cost time and memory in proportion to the functions they add (100 x 20 with real gomock: 11.3 s against 2.9 s for 6,500 more functions and 7x the summary elements, about 1.3 to 2x the cost per function of the production code). Findings are identical.
- **Candidates.** Under CHA then VTA a mock is a candidate of a dynamic call only if some code converts it to the interface and the value reaches the call. Mocks that are only defined are not candidates (synthetic: 0). In the real runs, dynamic candidates in packages outside the closure: memos 0 of 1,859, gin-vue-admin 0 of 602, cloudreve 1 of 8,951, gogs 22 of 1,064 (19 of 703 sites; `internal/mocks` and `dbtest`), Gitea 85 of 16,508 (33 of 6,601 sites). Candidates per site: 1 for about 60 % of the sites (Gitea 63 %, memos 66 %), 0 for 4 to 14 %, 2 to 8 for most of the rest.
- One real effect of generated mock code: every gomock method calls `m.ctrl.T.Helper()` (an interface call), so a gomock package adds one `dynamic_dispatch` uncertainty per mock method (4,200 in the 100 x 20 case) that disappears when the package is out of scope. An uncertainty-count effect, not a time one.
- `-pass-stats` / `packages_top` are not on main yet, so per-package re-analyses of the real targets were not taken (the synthetic ones are exact: 1 per function).

### 5. The case behind the report

No target at hand has a mock-heavy part outside its closure, or any large part (the biggest outside set is 1,759 lines). The user's case is not reproduced; the name of that project is needed (a module with generated mocks as own code, or with mock / test-helper packages of tens of thousands of lines).
