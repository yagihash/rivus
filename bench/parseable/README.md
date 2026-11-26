# parseable

A log and observability server (actix-web), the Rust benchmark: <https://github.com/parseablehq/parseable>
at `5c20b8e477be162238037e202cea1869d657017b`, the parent of the fixes of three advisories.

> **Exception, trusted code only.** The command below runs `rivus-rust` on the **host**, and that executes the target's `build.rs` and proc macros.
> `bench/compare.sh` does not do this: it reads third-party Rust (parseable included) only inside the container (`rivus debug extract -lang rust`, the default;
> `bench/README.md`, "Rust: third-party code is read in the container only"). Run the host command only on a checkout you trust; otherwise use the container.

```sh
git clone https://github.com/parseablehq/parseable && cd parseable && git checkout 5c20b8e477be162238037e202cea1869d657017b
mkdir -p /tmp/parseable-assets     # build.rs would fetch the UI from the network without it
LOCAL_ASSETS_PATH=/tmp/parseable-assets rivus-rust --no-deps . > parseable.jsonl    # about 70 s
rivus analyze -ir parseable.jsonl -rules /path/to/rivus/bench/parseable/rules.yaml > result.json   # a few seconds
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/parseable/truth-5c20b8e4.json result.json
```

The settings the results were measured with: `--no-deps` (the dependencies are not read; with them
the extraction takes 174 s and the findings are the same, 24 against 24 at the time), the rules in
`rules.yaml`, the default of `rivus analyze`. actix-web is built in now (`internal/model/builtin/rust-actix-web.yaml`);
`rules.yaml` still holds its sources and getters, as they were when the results were measured (the rows are the same).

Under the sandbox, `build.rs` of `tikv-jemalloc-sys` runs `/usr/bin/sed` and `/bin/expr`, which Seatbelt denies, so `cargo check` fails
(2026-11-27, macOS, base tried with the same arguments as `bench/compare.sh`: rc=1 after 34 s). So the check of the 7 `not_from_entry`
findings of parseable (ADR-058) cannot be taken under the sandbox.

Results are in `docs/adr/` (see `make docs-gen WHAT=adr`).

## Precision

The 25 sink sites that rivus reported were read by an independent reviewer (`review-5c20b8e4.json`):
5 are vulnerabilities (3 with a published advisory, listed in the file; the 2 others are not published, so their places are left out of this repository), 19 are tainted but stopped by a check the tool does not see (the stream has to
exist, its name is validated, the header is rewritten by the authentication), 1 is wrong. So "the
input reaches the sink" holds for 24 of 25, and "this can be exploited" for 5 of 25. One reviewer,
and some of its claims are marked unverified in the file.

## The shapes, in `rust-curated`

The flows of the sites (the redirect, the storage path, the alert target, and one client-chosen host) are kept as small fixtures in
`rust-curated/src/parseable.rs`, so that `go test ./...` (with `RIVUS_RUST`) notices when one is lost without
extracting parseable. Three are found; the alert target (stored in a global map and posted to by another function) is
`@miss(silent)`, as in the real project. See the last section of [`../MISSES.md`](../MISSES.md).
