# rustypaste

The Rust detection target: <https://github.com/orhun/rustypaste> (MIT) at `00292076e47b5f500906a97a1b5c17665270d907`, the parent of
the fix of CVE-2026-90774 (a path traversal in the upload). The fix is PR #623, the commit `ac05d552596af4a8429d80f30d11f67117ce02c8`
(`fix(upload): validate override filename`): the path of the stored file is built again with `util::safe_path_join` when the file
name comes from the request header.

It is a row of `bench/deps-targets.json` that is run by name (`make bench-passline TARGETS=rustypaste`), not in the default
`TARGETS` and not a row of the pass line (`decided: false`): the line is INFO. The row records the convergence and `scc_coarsened`
(see `bench/README.md`); `detect` (`mode: location`, the truth `bench/rustypaste/truth-00292076.json`, category `path_traversal`)
adds `detected`, per vulnerability, by place: a finding of the category with its source at the labelled source (the upload
handler) and its sink at the labelled sink (the call that creates the file). The fixed commit is read only when the pre-fix commit
is detected, and `quiet_after_fix` is recorded, not part of PASS/FAIL.

The truth and the `detect` row are the only knowledge of this target that the tool holds (the truth file here, the `detect` row of
`bench/deps-targets.json`), so they can be replaced as a unit.

## Not part of the line

"Does the finding disappear after the fix" (`quiet_after_fix`) is not read as a result for this target until `guards:` (#169)
exists: a guard such as `safe_path_join` is not read as a sanitizer yet, so the value is recorded and not judged (the same as
`bench/dart-archive/README.md`). The line stays INFO until it is decided.
