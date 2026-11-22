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

**Question.** A user reported scan times growing a lot "because of mocks". `./...` reads every package of the module as own code, also the ones that no non-test `main` imports (mock packages, test helpers). Estimate, before implementing anything, what it would change to analyse only the own packages in the import closure of the `main` packages (`go list -deps -test=false` over all `main` packages, intersected with `go list ./...`). This is a change of the analysis scope (results change), not a speed-up: the findings that disappear are listed first. Names (`mock`, `test`) are used only to describe the set, never to select it. Binary: main at `2b2dbe7`; rules as in `bench/compare.sh` (memos and cloudreve: built-in only).

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

## Measurement 3: where the summary elements and the time of the dependencies come from (bench/gcp and memos; 2026-11-21)

**Question.** The 985,000 summary elements of memos (AWS SDK) and the 1,030,000 of `bench/gcp` (Cloud Spanner + BigQuery) look alike, and in both the time is almost all in dependencies. The hypothesis to test: *the closures of all operations meet in one call of a helper (gax `invoke`; the smithy middleware), which makes false flows between operations and big summaries; if so, the fix is `CallOut` (docs/DESIGN-callouts.md: the helper's call of its function argument made concrete per caller), not how summaries are stored.*

**Method.** main at `84877f1`. Rules: `bench/gcp/rules-stored.yaml` for gcp (`rivus analyze -v -pass-stats -rules rules-stored.yaml ./...` in `bench/gcp`); built-in only for memos (the copy of the earlier measurements, `-no-project-rules`). Temporary diagnostic builds, **not committed**: a dump of every summary by output kind, element kind and source site, and of the candidates of every call of chosen functions; a count of analyses per function; pprof labels on chosen functions; and three switches (the Real elements are not written to package variables; `internal/valuecalls.Resolve` is skipped; and, in a copy of `bench/gcp`, `r.Context()` is `context.Background()`). **There is no `-func-values` flag on main**; the closest switch is `valuecalls.Resolve` (the function-value candidates that the engine finds itself; the Go extractor's own candidates stay). Load: the machine was shared (load average 15 to 31, other analyses running), so **wall time is not comparable between runs; the user CPU, the elements and the re-analyses are** (same binary and input: gcp 53 s and 96 s, memos 235 s and 569 s).

### 3.1 The two programs, side by side

| | bench/gcp | memos |
|---|---|---|
| functions / passes / re-analyses | 20,819 / 6 / 41,355 | 21,808 / 3 / 36,127 |
| `summary_elems` / `summary_steps` | 1,033,593 / 134.6 M | 987,412 / 72.7 M |
| largest summary / largest SCC | 2,758 / 860 | 3,542 / 531 |
| user CPU / max RSS | 231 s / 4.3 to 4.6 GB | 836 to 940 s / 11.0 to 12.7 GB |
| findings | 24 (the V cases, nothing else) | 7 |
| elements that are `Real` (from a source site) | **815,840 (79 %)**, in 1,203 functions (5.8 %) | 83,760 (8.5 %) |
| elements that are `Param` (symbolic: "what the caller passes") | 217,845 (21 %) | 903,652 (91.5 %) |
| where the mass is | grpc, spanner, monitoring, bigquery storage: 65 source sites, each in about 12,500 elements | 82.7 % of all elements in **508 functions (2.3 %)** with 200 or more output paths |

The two have **different causes**.

### 3.2 bench/gcp: the elements are not the closures; the time is not the elements

**(i) `invoke` and `refreshSubConns`.**

| function | summary elems (Real / Param) | outputs | analyses | wall ms (sum) | candidates of its dynamic and function-value calls |
|---|---|---|---|---|---|
| gax `invoke` | 1,775 (1,690 / 85) | 44 | 3 | about 1,000 | `call(ctx, settings)`: **97** (the closure of every API client method), `opt.Resolve`: 72, `(error).Error`: 573, `Retryer.Retry`: 4, `Sleep`: 1 |
| `grpc.invoke` | 1,020 (865 / 155) | 28 | 25 | 280 | 1 and 1 |
| `grpclb.lbBalancer.refreshSubConns` | 343 (313 / 30) | 15 | 28 | 8,300 | 8 calls of 4, 2, 4, 2, 4, 4, 1, 4 (the balancer `SubConn` / `ClientConn` interfaces) |

So the closures **do** meet in `invoke` (97 candidates in one call; they come from the Go extractor's VTA: switching `valuecalls.Resolve` off changes nothing in these numbers), but:

- **The meeting does not make the summary.** With the Real elements not written to package variables, the same `invoke` (the same 97 / 72 / 573 candidates) has **83** elements (1,775 before), `grpc.invoke` 215 (1,020), `refreshSubConns` 28 (343). The union of 97 closures costs about 80 elements. Each closure's own summary is already about 2,000 elements, nearly all of them the same 65 source sites, so the union adds nothing to them.
- **`refreshSubConns` is not a merge.** Its 8 calls have 4 candidates at most, its summary is 343 elements in 15 outputs. Its 28 analyses (against 3 of `invoke`) are probably the iterations of the 860-function SCC of the grpc balancer interfaces (not checked that it is a member). Its time is not its work: the pprof of its analyses (labels on the function) is **1.8 s of CPU against 8.3 s of wall**; the rest is the garbage collector and allocation (in the whole run 27 % of the CPU is `gcBgMarkWorker`, and `madvise` is 14 %). The `slow function` lines of `-v` (300 to 490 ms) overstate this function by 4 to 5 times; `NewHTTP2Client` is real work (5.4 s CPU of 8.2 s wall: `copyLoc`, `guardsOfChain`).

**Where the 65 source sites in a thousand summaries come from: package variables.** The 65 sites are the `*http.Request` parameters of the 65 handlers of the sample. `-trace-element` on one of them: its handler writes the taint into variables of `golang.org/x/net/trace` and `sync` in the first pass; the dependencies that read them, and then `credentials/alts/internal/handshaker/service.hsConnMap` (1,439 writes), the same variable of `s2a-go` (1,260), `grpc/internal/channelz.db` (895), `compute/metadata.projID` (673), `certprovider.provStore` (104), carry it into the results of the gRPC stack, and so into everything that calls it (`gax.Invoke`, `grpc.invoke`, `ClientConn.NewStream`, the balancers). 1,082 functions end with all 65 sites in their outputs.

**Ablation (diagnostic builds).**

| run | summary_elems | summary_steps | analyses | user CPU | max RSS | findings |
|---|---|---|---|---|---|---|
| main | 1,033,593 | 134.6 M | 41,355 | 231 s | 4.6 GB | 24 |
| `valuecalls.Resolve` off (the closest to `-func-values`) | 1,035,254 | 134.8 M | 41,270 | 231 s | 4.5 GB | 24, **the same** |
| the Real elements not written to package variables | **282,479 (-73 %)** | **17.4 M (-87 %)** | 39,557 | **162 s (-30 %)** | **3.5 GB (-24 %)** | 24, the same sinks; the source of 2 differs (below) |
| `r.Context()` made `context.Background()` in the sample | 909,251 (-12 %) | 119.4 M | 41,319 | 216 s | 4.3 GB | 24 |

Wall time of the third run: 46 s against 53 s (-13 %). Taking 73 % of the elements away takes 30 % of the CPU and 13 % of the wall time away: **the time of bench/gcp is mostly not in the elements** (the SCC iteration and the memory churn are the rest). `r.Context()` explains a part: the model `(*net/http.Request).Context: flows recv -> result` gives the context of every handler the taint of the whole request, and a context goes into every client call; making it clean removes the BigQuery handlers' share (65 sites become 55 in a summary) and 12 % of the elements. The Spanner handlers still reach the variables, probably through the statement they hand to the client (not traced).

**(ii) Does one operation's value mix into another's?** Yes in the summaries and in the "other sources" of 2 findings; no in the sink reaches.

- Sink reaches (`SinkReach`): 49 functions have some; **none has a Real element**. A Spanner source cannot reach a BigQuery sink through them.
- The 24 findings: every sink is reported with its own handler's source, and no decoy (`D*`, `F*`) is reported. Two findings (`ReadColumnByName`, `ReadToStruct`, to `fmt.Fprint`; the stored-data source) list **64 other sources** (other handlers, all `user_input`): the value read by `row.ColumnByName` / `ToStruct` carries the 65 sites of the summaries of the Spanner decoding functions. With the Real elements not written to variables they list none, and the source of the finding is the handler itself instead of its `$bound` twin. So these are false extra sources on a true finding, not false findings.
- The summaries of 3 of the handlers (the `PartitionQuery` cases) have all 65 sites in their outputs; the other handlers have 1 or 2 (their own).

**(iv) `valuecalls.Resolve` off.** gcp: elements +0.2 %, analyses -0.2 %, time and memory within the noise, findings identical. memos: elements 987,412 to 967,475 (-2 %), analyses 36,127 to 35,845, user CPU 940 to 878 s (the load differed), findings identical (7). The function values of both programs come from the extractor's VTA, not from the engine's own tracing.

**Verdict on the hypothesis, for bench/gcp: not the cause.** The meeting of closures exists (97 candidates in one call) and it costs about 80 elements. What makes the 1.03 M elements is the taint of the 65 handlers travelling through package variables of the gRPC stack, which the summaries of 1,082 functions then carry. `CallOut` would not change these numbers. The time is not in the elements either (-73 % elements, -30 % CPU, -13 % wall), and the per-function time that pointed at `refreshSubConns` is mostly the collector.

### 3.3 memos: the shape is the one of the hypothesis, through interface calls and not function values

**(iii) The smithy middleware.** The 985,000 elements are not the 83 k Real ones. They are **symbolic elements in the middleware methods**: `HandleDeserialize` 135 functions with 339 k elements, `HandleSerialize` 137 / 174 k, `HandleInitialize` 124 / 142 k, `HandleFinalize` 58 / 61 k, `HandleBuild` 15 / 26 k, `Step.HandleMiddleware` 6 / 13 k: **743 k of 987 k (75 %) in about 475 functions**; the 508 functions with 200 or more output paths hold 82.7 %. Each has 211 to 354 distinct output paths (`/*/AbortRuleId`, `/*/AcceptRanges`, `/*/Account`, ...: the field names of **the output types of all the operations**) times the input paths and parameters. The package `aws-sdk-go-v2/service/s3` alone is 525,724 elements (53 %).

What merges: one call in each of the five decorated handlers, `h.With.HandleX(ctx, in, h.Next)` (the stage is the interface-typed *field* `With` of the receiver, which the step's `HandleMiddleware` builds in a loop over its stages):

| call | candidates | summary of the method |
|---|---|---|
| `decoratedSerializeHandler.HandleSerialize` | 134 | 831 elems |
| `decoratedDeserializeHandler.HandleDeserialize` | 131 | 2,072 elems, 354 output paths |
| `decoratedInitializeHandler.HandleInitialize` | 121 | 691 elems |
| `decoratedFinalizeHandler.HandleFinalize` | 56 | 2,917 elems |
| `decoratedBuildHandler.HandleBuild` | 12 | 1,269 elems |
| `orderedIDs.Add` (`ider.ID`) | 385 | 7 elems |

Every operation has its own stage types (for example `awsRestxml_deserializeOpListParts`), so the candidates of the call are the stages of all the operations; each of them calls `next.HandleDeserialize` (2 candidates: the decorated handler and the last one), which comes back to the same call. The chain is **one strongly connected component** (the largest SCC of the run has 531 functions, and 508 functions hold the elements), so every member has the union of all the operations' output paths in its summary: the summary of `awsRestxml_deserializeOpListParts.HandleDeserialize` has 354 output paths and 2,508 elements, where its own operation has about ten fields. Elements are about (members of the SCC) x (the union of the operations) = 500 x 2,000. **That is the operation mixing in memos: the output of one operation's deserializer holds the fields of every other operation's.** It does not move a finding here (7 findings; the SDK holds no source), and the time has the same shape (the deserializers' analyses take up to 47 s of wall each, summed over their 20 analyses, at a load of about 20).

(Synthetic check: the `middleware` shape of `bench/go-scale` has 9 stages shared by 40 operations, so the candidate sets stay at 9; the real SDK has a stage type per operation, 131 candidates in the call. The synthetic shape misses memos because of that; one stage type per operation, each writing its own output type into `Result any`, would be the test.)

### 3.4 What to fix next, with the reason

1. **memos: narrow the candidates of an interface call whose receiver was built by a caller we can see, by what flows into the receiver's field (`With`).** This is `CallOut`'s idea (make the call concrete per caller), but its target is **not a function-value parameter**: the stage is an interface in a field of a struct that the step builds in a loop over a slice of stages. The design of docs/DESIGN-callouts.md (parameter `p`, path `""`) would not reach it; it needs the "element of a function value" step (paths) and the slice of stages as an element. First reproduce it in `bench/go-scale` (a stage type per operation, the wrapper with a `With` field), then measure the narrowing there. Evidence: 75 % of memos' elements in about 475 `HandleX` methods; 131 / 134 / 121 / 56 / 12 candidates in five calls; the same summary in 508 members of one SCC.
2. **bench/gcp: find why the SQL text and the context of a handler reach package variables of the dependencies, and what reads them back.** Evidence: 73 % of the elements, 87 % of the steps and 30 % of the user CPU disappear when the Real elements are not written to variables, with the same 24 findings, and two findings list 64 false "other sources". Not measured: whether reading only the elements that the variable's type can hold, or not carrying the taint of a whole `*http.Request` into `Context()`, is enough (the `Context` model alone is 12 % of the elements). The switch used here also stops the true shared-state flows (ADR-069), so it is a measurement and not a candidate change.
3. **Do not read the per-function wall time of `-v` as the cost of the function.** On `refreshSubConns` the CPU is 1.8 s of 8.3 s wall; the run spends 27 % of its CPU in GC marking and 14 % in `madvise` (heap growth and release). A `GOGC` / `GOMEMLIMIT` trial with the same results (not done here) and a CPU-time column in the `slow function` line would show whether the time of both programs is the collector's. Nothing here speaks for reading the dependencies less: models only would be a silent miss.

## Measurement 4: what the Dart framework-callback entries are made of (2026-11-21)

**Question.** The functions reached from the entries went from 16 % to 98 % (ADR-073). If `Overrides` were every override of a dependency's method, `toString`, `==`, `hashCode` and `compareTo` would be entries too, and `reachability` would no longer tell the two apart.

**Method.** The IR of the 6 `flutter/samples` apps and of LocalSend `app/` (`--no-deps`; the files of ADR-073's measurement, extractor output with `overrides`), read by a temporary test of `internal/sg` (not committed) that applies `FrameworkCallbacks` and `ReachableWithCallbacks` and variants of the entry set. Its numbers for the whole set are the ones of ADR-073 (403 of 411; 19,750 of 20,522).

**(i) The overridden types of the entries** (the number of overrides of the entry functions, by the type that declares the overridden member; an entry that overrides several is counted for each).

6 apps (77 entries; 411 functions): `State` 26 (`build` 19, `initState` 4, `didUpdateWidget` 1, `dispose` 1, `didChangeDependencies` 1), `StatelessWidget` 25 (`build`), `StatefulWidget` 19 (`createState`), `ModalRoute` 5 (`buildPage`, `buildTransitions`, `barrierColor`, `barrierLabel`, `maintainState`), `SingleTickerProviderStateMixin` 1, `Page` 1, `TransitionRoute` 1. That is all of them (7 types).

LocalSend `app/` (388 entries; 20,522 functions), top 20 by overridden type:

| count | declaring type | count | declaring type |
|---|---|---|---|
| 73 | flutter `StatelessWidget` | 7 | refena `BaseReduxAction` |
| 60 | flutter `State` | 7 | refena `AsyncGlobalActionWithResult` |
| 29 | wechat_assets_picker `AssetPickerTextDelegate` | 7 | window_manager `WindowListener` |
| 28 | flutter `StatefulWidget` | 6 | refena `BaseSyncNotifier` |
| 22 | dart_mappable `ClassCopyWithBase` (`$make`) | 5 | `SendSessionStateCopyWith` (generated, in the app's package) |
| 22 | refena `ReduxAction` (`reduce`) | 5 | `WebSendStateCopyWith`, `ServerStateCopyWith`, `SettingsTabVmCopyWith` (each) |
| 22 | refena `AsyncReduxAction` (`reduce`) | 4 | `ReceiveSessionStateCopyWith`, `SettingsStateCopyWith`, shared_preferences_platform_interface `SharedPreferencesStorePlatform`, flutter `SliverPersistentHeaderDelegate` (each) |
| 15 | refena `ReduxNotifier` (`init`, `initialAction`) | | |

**(ii) Object members.** **No entry comes from `Object`'s members.** The extractor leaves `dart:*` supertypes out of `overrides` (ADR-073, decision 2), so the `toString`, `==` and `hashCode` of a class that extends only `Object` are not there. None of the names `toString`, `==`, `hashCode`, `compareTo`, `noSuchMethod` is an overridden member of an entry in the 6 apps (0 of 77); in LocalSend 6 `toString` overrides exist (generated `*Mappable` mixins of the app's own package), and they are not entries because the overridden method is declared in the code that was read. Taking the Object-like names out of the entry set changes nothing, by construction and by measurement (entries / functions reached; the last two columns are variants of the entry set):

| app | functions | from entry | all callbacks | without Object-like members | only types of `package:flutter` | only lifecycle (`build`, `createState`, `initState`, `dispose`, `didChangeDependencies`, `didUpdateWidget`) |
|---|---|---|---|---|---|---|
| form_app | 76 | 18 | 13 / 74 | 13 / 74 | 13 / 74 | 13 / 74 |
| navigation_and_routing | 115 | 13 | 25 / 113 | 25 / 113 | 25 / 113 | 18 / 105 |
| platform_channels | 74 | 9 | 14 / 74 | 14 / 74 | 14 / 74 | 14 / 74 |
| date_planner | 87 | 15 | 14 / 83 | 14 / 83 | 14 / 83 | 14 / 83 |
| dynamic_theme | 31 | 5 | 6 / 31 | 6 / 31 | 6 / 31 | 6 / 31 |
| testing_app | 28 | 5 | 5 / 28 | 5 / 28 | 5 / 28 | 5 / 28 |
| 6 apps | 411 | 65 (16 %) | 77 / **403 (98 %)** | 77 / **403 (98 %)** | 77 / 403 | 70 / 395 (96 %) |
| LocalSend `app/` | 20,522 | 1,546 (7.5 %) | 388 / **19,750 (96 %)** | 388 / **19,750 (96 %)** | 168 / 14,579 (71 %) | 161 / 14,570 (71 %) |

**What it says.** The suspicion does not hold for the Object members: they are not entries, and 98 % stays 98 %. For the six small apps every entry is a lifecycle or a route method of Flutter. For LocalSend a quarter of the 96 % (5,171 functions, 25 points) comes from entries that are **not Flutter's**: refena's `reduce` / `init` and so on (about 80 overrides), dart_mappable's `$make` (22), `AssetPickerTextDelegate`'s getters (29), `WindowListener` (7), `SharedPreferencesStorePlatform` (4). Whether a package calls the overrides of its own types (a state-management library calls `reduce`; a picker reads its text delegate's getters) is the claim that the rule makes for every dependency; it is likely true for these, and it is not checked against the code of the dependencies, which `--no-deps` did not read. `reachability` still tells apart: 8 of 411 and 772 of 20,522 functions are not reached even with the callbacks, and a finding in code that nothing registers stays `not_from_entry`. No change is proposed.

## Measurement 5: what carries the 65 handler sources of bench/gcp into package variables (2026-11-21)

**Question.** After the stage-2 trial of docs/DESIGN-shared-state.md (not lifting sub-cells to the root) made `bench/gcp` worse, where does the spread come from? Hypothesis to test: not the shape of the registry (map / field / slice) but **a whole tainted carrier of the handler's source (`ctx`, a connection, an options struct)** that goes into every call of the gRPC stack. If so, constant map keys (stage 3) would not fix `bench/gcp`.

**Method.** main at `8834929`, `bench/gcp`, `rules-stored.yaml`, `rivus analyze -v -pass-stats`. A diagnostic build, **not committed** (a patch of about 40 lines in the commit of the global writes, `internal/taint`; it prints, for each Real origin, the first writes into package variables and the steps the writer took; the findings are the same as without it). The machine was shared (load 13 to 20): user CPU, elements and re-analyses are comparable, wall time is not.

### 5.1 The first step of each of the 65 sources

The 65 sources are the `*http.Request` parameters in 33 plain functions (handlers, and one handler closure) and 32 `$bound` method-value twins. **All 65 reach the same six package variables in the first pass** (not one after another over passes): `golang.org/x/net/trace.activeTraces` (path `/*/[]/*` and `/*/[k]`), `x/net/trace.completedTraces` (`/*/[k]`), `x/net/trace.traceFreeList` (`/*/[]/*`), `compute/metadata.projID` (`/*/v`, a field of the cached value; 61 of 65), `go.opencensus.io/stats/view.defaultWorker` (`/*/c/*`, 60), `sync.allPools` (`/*/[]/*`, 5). The one entered first (in the fixed order of the commit) is `metadata.projID` for 60 and `trace.activeTraces` for 5. (`[k]`, `[]`: the key side and the element side of a map or slice in the engine's notation; `v`, `c`: struct fields.) Every one is a non-constant place: a struct field, a slice, a map keyed by a trace name or a hash.

What carries the taint out of the handler (the first model propagation on the writer's steps, and the first call that leaves the sample), counted by origin:

| rank | carrier (type) | made tainted by | first call out of the sample (receiver) | origins |
|---|---|---|---|---|
| 1 | `context.Context` | the model `(*net/http.Request).Context`: `flows recv -> result` gives `ctx` the taint of the **whole request** | `(*spanner.txReadOnly).Query / QueryWithStats / QueryWithOptions / AnalyzeQuery` | 32 |
| 2 | `context.Context` | the same model | `(*spanner.Client).ReadWriteTransaction / BatchReadOnlyTransaction / PartitionedUpdate*` | 23 |
| 3 | `string` (`param(r)` = `r.URL.Query().Get("name")`, into `spanner.Statement.SQL` or the argument of `Update` / `BatchUpdate`) | `(*net/url.URL).Query`, `(net/url.Values).Get` | `(*spanner.ReadWriteTransaction).Update / BatchUpdate*` | 5 |
| 4 | `context.Context` | the same model | `(*bigquery.Query).Read` | 4 |
| 5 | `string` (the statement text) | `(*net/url.URL).Query` | `(*spanner.txReadOnly).Query` | 1 |

**The carrier of the first step is `ctx` for 59 of the 65** (the `Context` model); the string of the request for 6. This confirms the chain of the hypothesis for the first step. It does not say the `ctx` is the only way in: that is what 5.2 tests.

### 5.2 Diagnostic: `(*net/http.Request).Context` returns a clean value

The diagnostic build also marks the `Context` model `pure: true` (nothing flows). Not a proposal: a handler's `ctx` can carry `WithValue` data of the request in real code.

| | main (diagnostic build, switch off) | `Context` pure |
|---|---|---|
| `summary_elems` | 1,033,594 | 909,511 (-12 %) |
| `summary_steps` | 134.6 M | 119.3 M (-11 %) |
| re-analyses | 41,510 | 41,517 |
| user CPU | 220.5 s | 218.0 s |
| max RSS | 4.22 GB | 4.00 GB (-5 %) |
| findings | 27 | 27 |
| `other_sources_count` 64 | 3 | **0** (the 3 now list 54) |
| `other_sources_count` 2 | 24 | 24 |
| sources that reach a package variable | 65 | **55** (the 10 BigQuery sources are gone; all 55 Spanner remain) |

**It does not shrink.** -12 % of the elements and nothing in CPU: the hypothesis "the whole request in `ctx` is the cause" is **not** supported. It removes the BigQuery handlers (their only way was `ctx`) and 10 of 64 false other sources, nothing else.

### 5.3 What the remaining 55 are (the next candidate, by the same method)

With `ctx` clean, every one of the 55 enters through the **string of the request** (`param(r)`): the statement text, which is a real flow (the SQL text *is* the data). The first calls out of the sample: `txReadOnly` 24, `ReadWriteTransaction` 15, `Client` 6, `BatchReadOnlyTransaction` 6, `spanner.NewStatement` 4. In 42 of the 55 first paths the chain is the same: the statement goes into the request message (`ExecuteSqlRequest`: `GetSql` loaded, stored into the `preparedQuery` and put in the **bounded cache of the location router** (`boundedCache.Put` -> `container/list.PushBack`, unmodeled)), and a **receiver of the router and its caches** (`keyRangeCache`, `cachedGroup`, `endpointClientCache`) now holds the taint; `fillRoutingHint...` -> `warmPendingEndpoints` ranges over those cache entries and calls `endpointClientCache.Get` -> `createEndpointClient` -> `grpc.DialPool` -> the token source -> `ctx` made by `context.WithTimeout` -> `metadata.ProjectIDWithContext` -> `cachedValue.get` stores into `projID`. So the second carrier is **not a connection or an options struct handed down: it is the state of long-lived receivers (router caches), keyed by hashes and not constants, in which the statement of one handler is stored and from which the next call loads**. Constant keys (stage 3) would not separate these entries (`uint64` hashes, `list` elements), and `from:` on `Context` (stage 5) would not touch them.

Both carriers are inside the dependency, below the handler's call. In the elements: the 65 sources are in the summaries of 1,082 functions of the stack (Measurement 3). Not measured here: which of the cache stores, if cut, would drop the 55 (candidate diagnostics: the `boundedCache.Put` / `list.PushBack` store, or `warmPendingEndpoints`'s range over the cache).

### 5.4 Next candidates by the same method: the options and the connection (diagnostics, 2026-11-21)

Same build and rules, cumulative (each row keeps the switches of the rows above): the options carrier = `google.golang.org/api/option.WithEndpoint` made `pure` (the endpoint string that the router's key lookup hands to `createEndpointClient` is in the options slice of `DialPool`); the connection carrier = `(*grpc.ClientConn).Invoke / NewStream`, `grpc.invoke`, `grpc.newClientStream` and the `singleConnPool` / `connPool` `Invoke / NewStream` of `google.golang.org/api/transport/grpc` made `pure` (the request message goes into the stream, and the stream object is then tainted as a whole). Not proposals; each stops real flows.

| run | `summary_elems` | `summary_steps` | re-analyses | user CPU | max RSS | findings | `other_sources` 64 / 2 | sources that reach a package variable |
|---|---|---|---|---|---|---|---|---|
| main (switch off) | 1,033,594 | 134.6 M | 41,510 | 220 s | 4.22 GB | 27 | 3 / 24 | 65 |
| `Context` pure | 909,511 (-12 %) | 119.3 M | 41,517 | 218 s | 4.00 GB | 27 | 0 (3 list 54) / 24 | 55 |
| + `WithEndpoint` pure (options) | 920,535 (-11 %) | 122.0 M | 41,283 | 208 s | 4.26 GB | 27 | 0 (3 list 54) / 24 | 54 |
| + connection pure | **463,551 (-55 %)** | **66.6 M (-51 %)** | 40,029 | 190 s (-14 %) | 3.38 GB (-20 %) | 27 | 0 (3 list 42) / 24 | 43 |

(The CPU of the main row varies 220 to 236 s between two runs on the loaded machine; elements, steps and re-analyses do not.)

**Reading.** No single carrier explains the spread. `ctx` is the first step of 59 of the 65 sources, and 12 % of the elements; the options change nothing (the same route continues through other calls); the connection and the stream, which take the message and are then tainted as a whole object, are another 43 points of the elements (-55 % in all), and 43 sources still reach a package variable after the three cuts (the router cache route, 28 of them through `boundedCache`, and others). **What the carriers have in common is not a key (constant or not) and not `ctx`: they are long-lived objects of the dependencies (the connection and stream, the router and its caches, the options) into which the real data of the request (the statement text, the message) is stored as a field or an element, and which are then read whole by every other call that holds the object.** The elements in the 1,082 functions are the same 65 or fewer origins repeated. The false `other_sources` of the three `ReadColumns` / `ReadColumnByName` / `ReadToStruct` findings stay (42 of 64 are left after all three cuts).

**What it means for the design (for the advisor to decide; the design is not changed here).** (A) stage 3 (constant keys) does not fix `bench/gcp`: the places are fields, slices, and maps keyed by hashes or names. (C) as written (`from: recv.ctx`, a `param_type` source that does not taint a private `ctx`) takes the `ctx` part only: 10 of the 65 sources and 12 % of the elements, no change in CPU. The rest is a **whole-object read of a tainted long-lived receiver in library code**, which is what the stage-2 lift (keep the `Rel` of the sub-cell) is about, and which blew up when done on every output. So by the rule of the brief this is the case "neither", and (B) (keep `Rel` only where the sub-cells are few) is the candidate; but the measurement does not show (B) reduces `bench/gcp`, so `bench/gcp` should stay a measurement and not be an acceptance of stages 2 or 3. A small profile of the real shape is the next step: a message stored into a field of a long-lived receiver (a stream, a cache entry), then the receiver passed whole to a function that reads another field and writes a package variable, for N handlers.

## Measurement 6: two real Dart servers, with the dependencies and without, with `-resolve-func-values` off and on (2026-11-22)

Targets (the user allowed these two, with a limit of 10 minutes each; `pool.yaml`): `cocoon` (`app_dart`, ae9eb1be) and `dart-pad` (`pkgs/dart_services`, ad668d13).
Engine: main at 635cc6e (#126 merged: `-timeout` is shared by the rounds of `-resolve-func-values` and the taint analysis; the rounds may use half of it).
Rules: the built-in ones (no source of these targets matches, so a result with the dependencies has no finding). `-timeout 540s`. The code of the targets was not run
(`dart pub get` only; the Flutter packages of the workspaces were taken out of the root `pubspec.yaml` of the copies).

| target | flag | wall | peak memory | functions | findings | zero-candidate calls | rounds / supplemented |
|---|---|---|---|---|---|---|---|
| dart_services, with the dependencies | off | 585 s (stopped by `-timeout`) | 10.1 GB | 28,072 | 0 (1 pass) | | |
| dart_services, with the dependencies | on | 576 s (stopped by `-timeout`) | 13.1 GB | 28,072 | 0 | 999 | 1 round, stopped by its half of the limit, not used / 0 |
| cocoon, with the dependencies | off | 44 s (the analysis 17 s) | 1.8 GB | 15,075 | 0 (4 passes, converged) | 363 | |
| cocoon, with the dependencies | on | 325 s | 12.3 GB | 15,075 | 0 | 363 | 1 round, stopped by its half of the limit (270 s), not used / 0 |
| dart_services, `-no-deps` | off / on | 9 s | 1.4 GB | 349 | 6 / 6 (new 0) | 8 / 6 | 2 rounds / 2 |
| cocoon, `-no-deps` | off / on | 10 s / 12 s | 1.3 GB | 2,265 | 0 / **5** (new 5, lost 0) | 51 / 21 | 2 rounds / 30 |

**What looks wrong.** (1) The analysis of `dart_services` with the dependencies does not finish in 9 minutes with the flag off: the time is the dependencies
(`own_spent_ms` 410 ms against `dep_spent_ms` 6,077 s), `largest_scc` is 12,130 functions, `summary_elems` 1,011,666. The packages that carry the uncertainty notes
the most: `dart_style` 671, `dart_services` 515, `analyzer` 224. It has `analyzer` as a dependency (as `extractor/dart` does: 27,748 functions). (2) `cocoon` has no `analyzer`
and its plain analysis is light (17 s), but **one round of the function-value analysis takes more than 270 s: 16 times a plain analysis or more**. (3) Without the dependencies
the rounds are light (about 200 ms for `cocoon`).

**Supplemented calls (`-no-deps`).** `cocoon`: 30 calls got candidates (median 7, most 8, none at the limit of 32). The 5 new findings (all `ssrf`) go through one of them, the call
`server(request.toRequest())` of the entry of the server (the candidate `createServer$1` is the real callee), and then through `handler.service(request)`, a dynamic dispatch over the classes
of the handlers: one right supplement gives one finding per handler that has an outgoing HTTP call. Each has uncertainty notes (`unresolved_callee`, `dynamic_dispatch` 4 to 21,
`container_element_imprecision`). Whether each is a vulnerability needs the code of the handler: not judged here.

**Reduced to**: nothing yet. The cost of the dependencies is for the models that cut a heavy dependency (ADR-092): the table of the packages (functions, time, summary elements) is the
input; a small synthetic project with one huge dependency is the case for the gate.

