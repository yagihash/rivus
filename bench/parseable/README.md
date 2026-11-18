# parseable

A log and observability server (actix-web), the Rust benchmark: <https://github.com/parseablehq/parseable>
at `5c20b8e477be162238037e202cea1869d657017b`, the parent of the fixes of three advisories.

```sh
git clone https://github.com/parseablehq/parseable && cd parseable && git checkout 5c20b8e477be162238037e202cea1869d657017b
mkdir -p /tmp/parseable-assets     # build.rs would fetch the UI from the network without it
LOCAL_ASSETS_PATH=/tmp/parseable-assets rivus-rust --no-deps . > parseable.jsonl    # about 70 s
rivus analyze -ir parseable.jsonl -rules /path/to/rivus/bench/parseable/rules.yaml > result.json   # a few seconds
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/parseable/truth-5c20b8e4.json result.json
```

The settings the results were measured with: `--no-deps` (the dependencies are not read; with them
the extraction takes 174 s and the findings are the same, 24 against 24 at the time), the rules in
`rules.yaml`, the default of `rivus analyze`.

Results are in `docs/DECISIONS.md`.

## Precision

The 25 sink sites that rivus reported were read by an independent reviewer (`review-5c20b8e4.json`):
5 are vulnerabilities, 19 are tainted but stopped by a check the tool does not see (the stream has to
exist, its name is validated, the header is rewritten by the authentication), 1 is wrong. So "the
input reaches the sink" holds for 24 of 25, and "this can be exploited" for 5 of 25. One reviewer,
and some of its claims are marked unverified in the file.
