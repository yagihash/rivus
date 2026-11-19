# dart-pub

The Dart benchmark: <https://github.com/dart-lang/pub> at `5e24336e71a05a96b558d496953ebed223f6e40a`, the parent of
the fix (`26c6985c742593d081f8b58450f463a584a4203a`, "Normalize path before writing file when extracting tar.gz")
of GHSA-q739-79rh-vmvp / CVE-2026-27704: a zip slip in `extractTarGz`, where the name of a tar entry is joined to the
destination without being normalized.

```sh
git clone https://github.com/dart-lang/pub && cd pub && git checkout 5e24336e71a05a96b558d496953ebed223f6e40a
dart pub get                                              # Dart 3.13.6
dart run /path/to/rivus/extractor/dart/bin/rivus_dart.dart --no-deps . > pub.jsonl   # about 11 s
rivus analyze -ir pub.jsonl -rules /path/to/rivus/bench/dart-pub/rules.yaml > result.json   # about 1 s
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/dart-pub/truth-5e24336e.json result.json
```

The settings the results were measured with: `--no-deps` (with the dependencies pub's own `analyzer` package makes
the program large, and the flow is inside pub), the rules in `rules.yaml` (the entry name as the source; the file
functions of pub's `io.dart` as the sinks), the default of `rivus analyze`.

Result: 9 of the 9 labelled sites (`extractTarGz`, every use of the path before the fix) are found, with 8 more sites
reported that are not labelled: 5 are the helpers of `io.dart` that the same flow reaches, 1 is the Windows symlink
(`cmd /c mklink` with the path as an argument, not through a shell), 2 are in tests of the project that the builtin rules
match.

## What it does not show

- The labels are one reader's judgement, made from the fix (one change, so every use of `filePath` before it is a site
  of the same defect). No other advisory of the pub ecosystem is a taint flow of this kind (GitHub lists 14 reviewed
  advisories for pub, none in shelf, shelf_static, dart_frog, grpc or pub-dev).
- The fixed commit gives the same findings (shifted by two lines): `p.normalize` followed by `p.isWithin` is not a
  sanitizer that the rules know, so the benchmark measures recall on the vulnerable code, not that a fix is recognised.
- One advisory is one flow: this is a smoke test of the Dart extractor on a large real program (522 Dart files
  in the project (`--no-deps`); the extractor found and fixed four defects of its own on it: nested collection elements,
  null-aware elements, a named constructor and a method of the same name, and unnamed extensions), not a measure of precision.
