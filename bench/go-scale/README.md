# go-scale

The generator of the scale profiles (`gen/`), the measurement script (`measure.py`) and the expected numbers
(`expected/<profile>-w<workers>.json`, checked by `go test ./cmd/rivus -run TestScaleSynth`). The shapes in the
table of the main profiles are in `bench/README.md`, "The scale profiles". This file has the profiles that have
parameters to scale.

## `stagefield`: the shape of memos' AWS SDK, through an interface in a field

`go run ./bench/go-scale/gen -shapes stagefield -ops N -stages M -fields F -out DIR` (not part of `-shapes all`,
so the input of `bench/compare.sh --light` does not change). `middleware` shares 9 stage types among 40 operations,
so the candidates of `n.With.HandleMiddleware` stay 9 and it does not reproduce memos. `stagefield` gives **every
operation its own M stage types and its own output type with its own F field names** (a stage writes some of the
fields into `resp.Result`), and the stack builds `node{Next, With}` (the stage is an interface FIELD) in a loop. The
candidates of the one call are all N x M stage types; the chain is one strongly connected component; the union of
the N x F field paths goes into the summary of every member.

Measured on the head (`main` at `c07f360`; `measure.py`; the machine was shared, so time and memory are indicative,
the elements and re-analyses are exact). `summary_elems` is the number that `-pass-stats` prints. The ballast alone
has about 5,100 elements; the functions are the ballast (2,000) plus the shape; found 2 / missing 0 / false 0 in every row.

| ops | stages | fields | functions | summary_elems | largest summary | reanalyses | time | max RSS |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10 | 4 | 8 | 2,147 | 11,843 | 82 | 2,191 | 0.8 s | 0.13 GB |
| 20 | 4 | 8 | 2,257 | 31,393 | 162 | 2,341 | 1.2 s | 0.15 GB |
| 40 | 4 | 8 | 2,477 | 108,893 | 322 | 2,641 | 1.7 s | 0.31 GB |
| 80 | 4 | 8 | 2,917 | 417,493 | 642 | 3,241 | 3.7 s | 0.88 GB |
| 160 | 4 | 8 | 3,797 | 1,649,093 | 1,282 | 4,441 | 17.5 s | 3.3 GB |
| 40 | 4 | 16 | 2,477 | 212,253 | 642 | 2,641 | 2.4 s | 0.50 GB |
| 40 | 9 | 12 | 2,877 | 352,973 | 482 | 3,241 | 3.4 s | 0.78 GB |
| 80 | 9 | 12 | 3,717 | 1,392,053 | 962 | 4,441 | 16.3 s | 2.7 GB |

It **reproduces**: the elements are proportional to fields (ops 40: 8 fields 108,893, 16 fields 212,253) and to the
square of ops (the members of the SCC grow with ops, and so does the union): x4 for every doubling of ops; the largest
summary is ops x fields x 1 (the union). ops 80 / stages 9 / fields 12 has 1.39 M elements, as many as memos
(987,412), at 16 s. One pass, converged, findings unchanged: the swelling is in the size of the summaries, not in
the answer. The expected file `expected/stagefield-w1.json` is ops 40 / stages 4 / fields 8.

What it does not reproduce: memos' time (the dependencies' own analyses, minutes) and its 3 passes; the field names
here never overlap between operations (in the SDK many names are shared, so the union is smaller than ops x fields).
