# dart-archive

The Dart detection target: <https://github.com/brendan-duncan/archive> (package:archive) at `6fc3b89fcdb89b593402e0dabc13a7d7fd3f762d`
(3.3.7, the last affected version of GHSA-9v85-q87q-g4vg / CVE-2023-39139). The fixed version is 3.3.8
(`01110f00e0d0b9b2bc524dd074b896c5409e54db`; the fix is `edb0d480733a44d28ff3d5e4e2779153ba645ce7` and the follow-up
`6de492385d72af044231c4163dff13a43d991c83`): `extractArchiveToDisk` created a symbolic link whose target is the name stored in
the archive, with no check that it stays inside the output directory.

It is a target of the pass line (`bench/deps-targets.json`, `make bench-passline TARGETS=dart-archive`): `git clone` and
`dart pub get` only (the code of the target is never run), default flags, builtin models only (the entry-name sources of
`package:archive`, `internal/model/builtin/dart-archive.yaml`; they need the 3.x layout rows of that file).
`detected` is per vulnerability: a finding passes through a labelled position (`truth-6fc3b89f.json`: the two calls that create the
link with the stored target). The line is INFO until it is decided.

## What was measured (2026-11-23)

- The old code (Dart 2.12 era, `sdk: '>=2.12.0 <3.0.0'`) is analysed with the current SDK: `dart pub get` resolves and the
  extractor reads it. One run: 11 s, 1.14 GB, converged, 2290 functions.
- Findings of `path_traversal`: 65 at 3.3.7 and 73 at 3.3.8. The entry name (`ArchiveFile.name`) and the link target
  (`ArchiveFile.nameOfLinkedFile`) reach `File.new` and `Link.new` in `extract_archive_to_disk.dart` and the file streams. They are
  not quiet after the fix: rivus does not read a guard (`isWithin`, `_isValidSymLink`) as a sanitizer, so the fixed version reports
  as much (recorded; not part of the line).
- The labelled flow itself (the stored name as the **target** of `Link.create`) was not found: `dart:io#Link.create` had no sink for
  the target argument (the builtin rows had `Link.new`, the path of the link). So `detected=no`, `through_tp=0/2`.

## After the sink rows of `Link.create` / `Link.createSync` (2026-11-25)

- `internal/model/builtin/dart-io.yaml` has the target (argument 0) of both methods as `path_traversal` sinks; the zoo cell is
  `dart/io-link-create-target` (dart:io cannot be recorded by the run, so the cell gives the same value to `sinkPath` on the line).
- `detected=yes`, `through_tp=2/2`, 11 s, converged (still INFO). The finding ends at the labelled call itself (its last step is
  `sink: dart:io#Link.create`, not a `call` step), so `through_counts` of `bench/deps_bench.py` counts a `sink:` step at the
  labelled place as well as a `call` step.
- At 3.3.8 the same sites are still reported (the guard is not read as a sanitizer, as above). A finding there carries no
  uncertainty that points at the added check; `numeric_stop` entries that name `_isWithinOrEquals` (the check) are in the report,
  but they are not attached to the findings. Not part of the line.
