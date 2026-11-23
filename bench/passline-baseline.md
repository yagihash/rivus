## The pass line (`make bench-passline`)

The goal: rivus analyses a real product with the default flags, in a realistic time and memory, with a meaningful result. One line per target (bench/deps_bench.py; the targets, the pinned commits and the limits are data in bench/deps-targets.json):

`PASS|FAIL memos time=<s>/<limit> mem=<GB>/<cap> converged=<yes|no> detected=<yes|no|n/a>`

PASS for memos (usememos/memos 26.11.2, `00fcd170281a75fc0e95ec4f2f54f43d32fe81bd`) needs all four with the default flags (no `-no-deps`, no `-body-replaced`): it ends by itself within 180 s; the memory of the process group stays within 8 GB (a watchdog kills the group of the run above the cap); it converges; and CVE-2025-65799 (path traversal; fixed in 0.25.3) is detected: the count of `path_traversal` findings is higher on the pre-fix commit `1d7efb158045924ab87f58d567bd79cd92edf6af` than on the fixed commit `5f57f48673e2054f404b2c5b497a8eaa3690591d` (counts only). The lines of dart_services (dart-pad `pkgs/dart_services`, `ad668d137e631beeaef3e263054a45696e8d4499`) and cocoon are not decided yet; dart_services is printed as `INFO`.

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
