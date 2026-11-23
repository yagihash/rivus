## The pass line (`make bench-passline`)

The goal: rivus analyses a real product with the default flags, in a realistic time and memory, with a meaningful result. One line per target (bench/deps_bench.py; the targets, the pinned commits and the limits are data in bench/deps-targets.json):

`PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> detected=<yes|no|n/a>`

PASS for memos (usememos/memos 26.11.2, `00fcd170281a75fc0e95ec4f2f54f43d32fe81bd`) needs all four with the default flags (no `-no-deps`, no `-body-replaced`): it ends by itself within 180 s; the end-of-run peak memory stays within 8 GB (a watchdog that samples the process group stops a run above the cap); it converges; and CVE-2025-65799 (path traversal; fixed in 0.25.3) is detected: the count of `path_traversal` findings is higher on the pre-fix commit `1d7efb158045924ab87f58d567bd79cd92edf6af` than on the fixed commit `5f57f48673e2054f404b2c5b497a8eaa3690591d` (counts only). The Dart line is cocoon `app_dart` (below); dart_services (dart-pad `pkgs/dart_services`, `ad668d137e631beeaef3e263054a45696e8d4499`) is a load target, printed as `INFO`.

**The next form of the "after the fix" condition (decided 2026-11-23; not in force yet).** The condition above counts findings of the category, which says nothing about whether the finding went away for the right reason. Once the mechanism of issue #169 exists (a check is not deleted from the findings: the finding is kept and says which check function was passed on which branch), the detection condition becomes: before the fix, a finding appears and has no check note; after the fix, there is no finding, or there is a finding that carries a note pointing at the check function that the fix added. Until that mechanism exists, the line above is the condition, and a result of "no finding after the fix" (`quiet_after_fix`, where the runner has it) is only recorded, not part of PASS or FAIL.


### The starting point (2026-11-23, numbers only, a local measurement of the engine of that day)

| target | commit | flags | wall | max memory | converged | functions | findings |
|---|---|---|---|---|---|---|---|
| memos | 00fcd170 | default, cap 8 GB | cut by the cap at 67 s | > 8 GB | no | - | - |
| memos | 00fcd170 | `-timeout 50s` (partial) | 99.5 s (CPU 178 s) | 6.75 GB | no | 40.8k | not meaningful (partial) |
| memos | 00fcd170 | `-body-replaced` of the aws SDK (NOT the default flags; informational) | 45 s | 6.08 GB | yes | 34.7k | - |
| dart_services | ad668d13 | default, cap 8 GB | cut by the cap at 189 s | > 8 GB | no | - | - |
| dart_services | ad668d13 | `-timeout 50s` (partial) | 138 s | 7.98 GB | no | 28.1k | 0 |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | default, cap 8 GB | cut by the cap at 49 s and 31 s | > 8 GB | no | - | - |
| memos CVE-2025-65799, pre-fix and fixed | 1d7efb15, 5f57f486 | `-body-replaced` of the aws SDK (informational) | 30 s and 23 s | 3.7 GB and 3.6 GB | yes | - | 0 and 0 (`path_traversal`; not detected at the pre-fix commit) |

So today's line for memos is `FAIL memos time=67/180 mem=>8/8 converged=no detected=no` (cut by the cap).

### The cocoon line (2026-11-23, numbers only; engine of that day)

The Dart target of the pass line is flutter/cocoon `app_dart` (`ae9eb1beaf765d644cc85dde38def030c80b4b7d`). Launch: cwd = `app_dart`, `-allow-deps-under <cocoon>/packages`, `dart pub get` only, the Flutter member of the root workspace removed in the copy (all in `bench/deps_bench.py`; data in `bench/deps-targets.json`). The line is the same as for memos (default flags, within 180 s and 8 GB, converged): PASS for time, memory and convergence. `detected=n/a`: the project has no public security advisory, so no pre-fix / fixed pair. dart_services stays a load target for the heavy dependency `package:analyzer` (printed as `INFO`, numbers kept; also a target of the depth experiments).

| target | commit | flags | wall | max memory | CPU | converged | functions | findings |
|---|---|---|---|---|---|---|---|---|
| cocoon app_dart | ae9eb1be | default, 10m | 101 s | 1.78 GB | 128 s CPU | converged (4 passes) | 16.5k functions (14.3k deps) | 14 findings (ssrf 9, resource_name 4, sql_injection 1) |

Today's line for cocoon: `PASS cocoon time=101/180 mem=1.78/8 converged=yes detected=n/a`.

### Detection: the line is per vulnerability, the detail is per position

`detected` counts per vulnerability, as for memos: `yes` when at least one finding passes through a labelled position of that vulnerability's truth (builtin models, default flags). The position-level count is printed beside it as information: `through_tp=<n>/<N>`. For the Dart target `dart-pub` (dart-lang/pub `5e24336e71a05a96b558d496953ebed223f6e40a`; one public fixed vulnerability, the zip slip of `extractTarGz`, 9 labelled call sites; `bench/dart-pub`), the position count is 4/9 with `-no-deps` and the builtin models (an earlier local measurement). With the default flags the run does not converge today, so the line is `detected=no` (INFO; the line is not decided). `bench/score.py --through` gives the same position count from a saved result.


### Peak memory: what was measured how (numbers only)

The memory of the verdict is the END-OF-RUN PEAK of the child (`/usr/bin/time` max resident set; macOS reports bytes). The watchdog (a sample of the summed RSS of the process group, which kills the group of the run above the cap) only stops a run early; its samples are never the number of the verdict (a sampler can miss a short peak: a run was seen above 8 GB that the sampler did not see). A run that the watchdog killed has no end-of-run number and is recorded as "above the cap".

| commit | setting | peak memory | how measured |
|---|---|---|---|
| memos 00fcd170 (the pass-line target) | `-dep-k 2` | 8.27 GB | sampled, loaded machine (earlier) |
| memos 00fcd170 | `-dep-k 2` | 8.56 GB | sampled, summed RSS of the process group (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.2 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 1` | 7.8 GB | sampled, summed RSS of the process group (earlier) |
| memos 1d7efb15 (the older pre-fix commit of the detection check, NOT the target) | `-dep-k 2` | 5.9 GB | sampled (earlier) |
| memos 00fcd170 | `-dep-k 2` | 6.35 GB (163 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |
| memos 00fcd170 | `-dep-k 1` | 5.49 GB (575 s, converged) | end-of-run peak, 8 GB watchdog, limit 10 min (2026-11-23, engine fb492ef7) |

The 5.9 GB figure belongs to a different commit than the target; do not compare it with the 00fcd170 rows. The earlier rows were sampled, on an older engine and a loaded machine; the two last rows are the first end-of-run peaks of the target (the wall time of the second one was long: a loaded machine, not measured separately).
