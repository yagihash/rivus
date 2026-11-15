# Benchmarks

Ground truth for open-source targets, and a scorer. The targets are not vendored: check them out
next to this repository, run `rivus analyze`, and score the result.

```sh
git clone https://github.com/Contrast-Security-OSS/go-test-bench && cd go-test-bench
rivus analyze ./... > result.json
python3 /path/to/rivus/bench/score.py /path/to/rivus/bench/go-test-bench/truth.json result.json
```

A finding is matched with a labelled sink site by (file, line, category). The labels were made by
reading the code (the targets have no answer key), and the judgement calls are in each entry's
`note`: the truth file is a claim, and anyone can read and dispute it.

Results are recorded in `docs/DECISIONS.md` (ADR-043).
