## The pass line (`make bench-passline`)

The goal: rivus analyses a real product with the default flags, in a realistic time and memory, with a meaningful result. One line per target (bench/deps_bench.py; the targets, the pinned commits and the limits are data in bench/deps-targets.json):

`PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> finish=<yes|no> detected=<yes|no|n/a> quiet_after_fix=<yes|no|n/a> located=<n>`

The pass line is two separate conditions, and PASS needs both (decided 2026-11-24). The default flags are used (no `-no-deps`, no `-body-replaced`).

1. **It finishes** (`finish`), for memos (usememos/memos 26.11.2, `00fcd170281a75fc0e95ec4f2f54f43d32fe81bd`): it ends by itself within 180 s; the peak memory of the whole process tree (the same-time sum, sampled every 0.25 s) stays within 8 GB (the watchdog stops a run above the cap); and it converges (`converged`; no function stopped on the clock).
2. **It detects** (`detected`): CVE-2025-65799 (path traversal; public, fixed in 0.25.3) is found at the pre-fix commit `1d7efb158045924ab87f58d567bd79cd92edf6af` by place: at least one `path_traversal` finding has its source and its sink at the labelled places of the truth file [bench/memos/truth-1d7efb15.json](memos/truth-1d7efb15.json) (builtin models; `located=<n>` is the number of such findings, information). The truth file is read from the public fix (one file, `server/router/api/v1/attachment_service.go`): 1 source place (the request of `CreateAttachment`, line 51) and 3 sink places (`os.MkdirAll` 416, `os.Create` 419, `os.WriteFile` 426 in `SaveAttachmentBlob`). The lines are those of the pre-fix commit; the same sinks at the fixed commit are 422, 425 and 432 (the fix adds two checks before them), kept in the same file as `fixed_line`. A finding of the same kind at other places does not count. The two conditions are judged apart: `finish` does not use the detection, and the detection is read only from a run that converged.

`-no-deps` also meets the detection (a measured fact, 2026-11-24: memos at the pre-fix commit, `-no-deps`, 3 `path_traversal` findings at the 3 sink places, 5.7 s, 653 MB), so the detection does not depend on the dependencies; "finishes with the dependencies" is the other condition and is the one that the default flags have to meet.

`quiet_after_fix` (no `path_traversal` finding at the fixed commit `5f57f48673e2054f404b2c5b497a8eaa3690591d`) is a separate indicator and is not part of PASS/FAIL: rivus does not yet treat a fix of the kind "validate, then return early" as removing the flow (the declared checks of #169 do not exist yet). The fixed commit is analysed only when the pre-fix commit is detected; otherwise it is `n/a`. For a target with a truth file of the other kind (`through_tp`, dart-pub), `detected` is per vulnerability as below and `quiet_after_fix` is `n/a`. The Dart line is cocoon `app_dart` (below); dart_services (dart-pad `pkgs/dart_services`, `ad668d137e631beeaef3e263054a45696e8d4499`) is a load target, printed as `INFO`.

In the ledger ([heavy-ledger.tsv](heavy-ledger.tsv); `python3 bench/heavy_ledger.py`) the two are separate: `finish` (time, memory, `conv`) and `det`; `result` is PASS when both are yes.

**The next form of the "after the fix" condition (decided 2026-11-23, changed 2026-11-24 by ADR-122; not in force yet).** The condition above counts findings of the category, which says nothing about whether the finding went away for the right reason. Once the declared checks of issue #169 exist (a rule file declares a check function with `guards:`, and a finding that passes it is stopped, with a record in `suppressions` that names the rule file and the declaration), the detection condition is run with a rule file that declares the check that the fix added, on both commits: before the fix, with that rule, a finding still appears (the check does not exist yet there); after the fix, with that rule, there is no finding, and `suppressions` names the declaration that stopped it. Without the declaration the finding at the fixed commit stays, which is the evidence that the fix is the check. Until that mechanism exists, the line above is the condition, and a result of "no finding after the fix" (`quiet_after_fix`, where the runner has it) is only recorded, not part of PASS or FAIL.

### The two lines of the pass line: A and C (decided 2026-11-24; both not measured yet)

The user decided that the pass line is two lines, A and C, for different reasons (each was heavy for a different reason). The memos line above (8 GB) stays as it is. The earlier decision "Gitea is not used for the pass line" (2026-11-21) is replaced by this one: Gitea is fetched and run as a target of the pass line. Periodic runs and exploration on it are a separate decision. The result of each is in the ledger (`python3 bench/heavy_ledger.py --kind passline --target <name>`); no number is copied here.

| line | target | flags | time | peak of the tree | other conditions |
|---|---|---|---|---|---|
| A | `memos-4gib`: memos `00fcd170` | the default flags and `-max-memory 4GiB` (the default limit of a machine with 16 GB of physical memory, ADR-121: the same conditions as that machine) | 180 s | 4 GiB | converged; detected (the same places as the memos line) |
| C | `gitea`: Gitea `v1.26.4` (`95ba37d9`, `go-gitea/gitea`) | the default flags, with the dependencies, `-no-project-rules` | 600 s (the limit of the earlier runs, 10 minutes; the 180 s of memos has no ground for Gitea) | 8 GB | converged; `detected=<n>/3` is printed but is **not** part of PASS/FAIL (and will not be: it is for reference) |

The detection of C is per published vulnerability of [gitea-cve-truth.json](gitea-cve-truth.json), judged by place (the form of `bench/score.py`). C was added for its size, so the detection is not in the verdict, now or later: it is printed for reference only. A regression of a memory change is checked by the sameness of the graph representation, not by the count. The line and the ledger row of C carry the count `n/3` only: no count of findings, no list, no path, no place. `bench/deps-targets.json` has both targets; the runner reads `detect.mode` `score` (the field `vuln` of the truth file groups the entries of one vulnerability). `gitea` is not in the default `TARGETS` of `make bench-passline` (it takes up to 10 minutes): `make bench-passline TARGETS=gitea`. The time limit is kept by the watchdog of the runner; rivus is not given `-timeout` (its default is no limit), so a run that is cut at the limit is `stopped_time` and not a run that did not converge. The fetch (`git clone`, `git worktree`, `go mod download` into `~/.cache/rivus-bench`) never runs a script or `go generate` of the fetched code. Run it with the heavy-work lock, not at the same time as another heavy run.

### The cocoon target

The Dart target of the pass line is flutter/cocoon `app_dart` (`ae9eb1beaf765d644cc85dde38def030c80b4b7d`). Launch: cwd = `app_dart`, `-allow-deps-under <cocoon>/packages`, `dart pub get` only, the Flutter member of the root workspace removed in the copy (all in `bench/deps_bench.py`; data in `bench/deps-targets.json`). The line is the same as for memos (default flags, within 180 s and 8 GB, converged). `detected=n/a`: the project has no public security advisory, so no pre-fix / fixed pair. dart_services is pass line D (below; also a target of the depth experiments).

Pass line D: dart_services (user decision, 2026-11-24; dart-lang/dart-pad `pkgs/dart_services` at `ad668d137e631beeaef3e263054a45696e8d4499`, default flags). Conditions: converged, tree-total peak within 8 GB, time within 600 s (the same reason as Gitea in C: the 180 s had no ground). `detected=n/a` (no public vulnerability; not part of PASS/FAIL). `decided` is true. It takes up to 10 minutes, so it is not in the default of `make bench-passline`; select it with `TARGETS=dart_services`. The result now is FAIL: the `dart_services` row of the ledger with `stopped_memory` (stopped by the 8 GB watch; `python3 bench/heavy_ledger.py --target dart_services --last 1`). The cause is that the summaries in the huge SCC do not converge (#126 fixed only the deadline). The next work is the first measurement per iteration inside the SCC (`RIVUS_INTERNAL_SUMMARY_DUMP`). A one-time investigation run may use a larger cap (10 GB); that value is not the pass line. See `docs/ROADMAP.md` section 3.

### Detection: the line is per vulnerability, the detail is per position

`detected` counts per vulnerability, as for memos (which labels the source and the sink places; dart-pub labels the call sites a finding passes through): `yes` when at least one finding passes through a labelled position of that vulnerability's truth (builtin models, default flags). The position-level count is printed beside it as information: `through_tp=<n>/<N>`. The Dart target `dart-pub` (dart-lang/pub `5e24336e71a05a96b558d496953ebed223f6e40a`; one public fixed vulnerability, the zip slip of `extractTarGz`, 9 labelled call sites; `bench/dart-pub`) is printed as INFO (its line is not decided). `bench/score.py --through` gives the same position count from a saved result.

### Peak memory: what is measured how

The memory has two records (`bench/deps_bench.py`). (a) the end-of-run peak of the main process (`ru_maxrss` of the child from `wait4`: the largest single process of its subtree; bytes on macOS). (b) the whole PROCESS TREE: the maximum over fine samples (every 0.25 s) of the SAME-TIME SUM of the RSS of all its processes (shared pages can be counted twice); the biggest process of the tree at that moment and when are kept, so that a large (a)/(b) difference can be explained. The verdict (PASS/FAIL) uses (b): the burden the machine really receives. The watchdog (the same samples; it kills the group of the run only, above the cap or the time limit) only stops a run early.

### Where the numbers are

Every run of `make bench-passline` and `make bench-deps` adds one row to [heavy-ledger.tsv](heavy-ledger.tsv) (the columns, the notes: [heavy-ledger.md](heavy-ledger.md); `python3 bench/heavy_ledger.py` prints it with the margins). A number in a PR or an ADR points to a row (date and commit); it is not copied. `NO_LEDGER=1` adds no row (a binary with a measurement patch, or a run that is not used as a number). `BENCH_OUT` (default `.claude/work/bench-out/`, git-ignored) holds the JSON of the results; the ledger holds numbers only.

## Records before 2026-11-23 (not in the ledger)

These rows were measured before the ledger existed (by hand, with the engine of that day; the commit of rivus is not recorded, and the way of measuring differs from the ledger's). They are kept as they are, numbers only. None was moved to the ledger.

### The starting point (numbers only, a local measurement of the engine of that day)

| target | commit | flags | wall | max memory | converged | functions | findings |
|---|---|---|---|---|---|---|---|
| memos | 00fcd170 | default, cap 8 GB | cut by the cap at 67 s | > 8 GB | no | - | - |
| memos | 00fcd170 | `-timeout 50s` (partial) | 99.5 s (CPU 178 s) | 6.75 GB | no | 40.8k | not meaningful (partial) |
| memos | 00fcd170 | `-body-replaced` of the aws SDK (NOT the default flags; informational) | 45 s | 6.08 GB | yes | 34.7k | - |
| dart_services | ad668d13 | default, cap 8 GB | cut by the cap at 189 s | > 8 GB | no | - | - |
| dart_services | ad668d13 | `-timeout 50s` (partial) | 138 s | 7.98 GB | no | 28.1k | 0 |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | default, cap 8 GB | cut by the cap at 49 s and 31 s | > 8 GB | no | - | - |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | `-body-replaced` of the aws SDK (informational) | 30 s and 23 s | 3.7 GB and 3.6 GB | yes | - | 0 and 0 (`path_traversal`; not detected at the pre-fix commit) |

So today's line for memos is `FAIL memos time=67/180 mem=>8/8 converged=no detected=no quiet_after_fix=n/a` (cut by the cap; not detected at the pre-fix commit with the default flags and the builtin models).

### The cocoon line (numbers only; engine of that day)

| target | commit | flags | wall | max memory | CPU | converged | functions | findings |
|---|---|---|---|---|---|---|---|---|
| cocoon app_dart | ae9eb1be | default, 10m | 101 s | 1.78 GB | 128 s CPU | converged (4 passes) | 16.5k functions (14.3k deps) | 14 findings (ssrf 9, resource_name 4, sql_injection 1) |

Today's line for cocoon: `PASS cocoon time=101/180 mem=1.78/8 converged=yes detected=n/a`.

### Peak memory (numbers only)

The rows marked end-of-run were measured with the earlier method (`/usr/bin/time`, the largest single process, = (a) in the naming above). The earlier rows were sampled, on an older engine and a loaded machine.

| commit | setting | peak memory | how measured |
|---|---|---|---|
| memos 00fcd170 (the pass-line target) | `-dep-k 2` | 8.27 GB | sampled, loaded machine (earlier) |
| memos 00fcd170 | `-dep-k 2` | 8.56 GB | sampled, summed RSS of the process group (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.2 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.8 GB | sampled, summed RSS of the process group (earlier) |
| memos 1d7efb15 (the older pre-fix commit of the detection check, NOT the target) | `-dep-k 2` | 5.9 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 2` | 6.35 GB (163 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |
| memos 00fcd170 | `-dep-k 1` | 5.49 GB (575 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |

The 5.9 GB figure belongs to a different commit than the target; do not compare it with the 00fcd170 rows. The two last rows are the first end-of-run peaks of the target (the wall time of the second one was long: a loaded machine, not measured separately).

### The position count of dart-pub

4/9 with `-no-deps` and the builtin models (an earlier local measurement). With the default flags the run did not converge then, so the line was `detected=no` (INFO).
