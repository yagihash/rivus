# Benchmarks

Ground truth for open-source targets, and a scorer. The targets are not vendored: check them out
next to this repository, run `rivus analyze`, and score the result.

```sh
git clone https://github.com/Contrast-Security-OSS/go-test-bench && cd go-test-bench
rivus analyze -rules /path/to/rivus/examples/rules/go-gin.yaml ./... > result.json   # the rules the target needs
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/go-test-bench/truth.json result.json
```

A finding is matched with a labelled sink site by (file, line, category). The labels were made by
reading the code (the targets have no answer key), and the judgement calls are in each entry's
`note`: the truth file is a claim, and anyone can read and dispute it.

Results are recorded in `docs/DECISIONS.md` (ADR-043).

## Benchmarks that are in this repository

The targets above are real projects: they are big, slow to analyse, and have no answer key but the
labels made by hand. For the work on the engine there are also two small benchmarks of our own, made
for each language (`go-*` now; `dart-*` and `rust-*` are to be put next to them in the same way):

| directory | what it is | what it answers | time |
| --- | --- | --- | --- |
| `go-curated/` | a project in the shape of Gitea, gogs, gin-vue-admin, ech0, with the flows to find and the ones not to find written in the source | did the accuracy change (a flow lost, a false one gained)? | seconds, in `go test ./...` |
| `go-synth/` | a generator of Go code of a given shape (depth, SCCs, interfaces, fields, ...), with a script that measures time, memory and the result | did the time or the memory change, and is the result the same? | S seconds, M minutes |

When the core of the engine (`internal/sg`, `internal/sites`, `internal/taint`) is touched: run
`go test ./...` (this includes `go-curated`), then the S and M of `go-synth` before and after with the
same binary built from each state, and check that the result is identical (or that every changed
finding is understood). A real project (Gitea) is for a milestone only: it takes ten to sixteen
minutes and 5 to 10 GB. See `go-synth/README.md` for how, and `docs/DECISIONS.md` (ADR-046) for why.
