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

## `registry`: false "other sources" through a registry in a package variable

`go run ./bench/go-scale/gen -shapes registry -ops N -out DIR` (by name only; not part of `-shapes all`). The shape of
`bench/gcp` (the notes of `bench/explore/LOG.md`, measurement 3): N handlers take a request (a source by its type,
`*reg.Req`), and each writes its body into one registry (`var M = map[string]string{}`, a package variable) under a
key of its own. Handler 0 also decodes its body (`Decode(s) = s + M["codec"]`) and sends it to the sink. One in eight
functions of the ballast calls `Decode`, so what the decoder carries goes up the call graph. The key is a constant and
the elements of a map do not say which key they came from: the decoded value carries the requests of all handlers, and
the one real finding (handler 0 to the sink) lists the other N-1 handlers as `other_sources`. None of them was ever in
the decoded value. The code has no answer key; `expected/registry-w1.json` (N = 40) is the check: the one finding,
`other_sources_count` 39, `summary_elems` 22,173. The 39 is a false trace that the test pins (it must not grow), and the profile skips `checkCuratedReport` because the code has no answer key: when stage 2 of `docs/DESIGN-shared-state.md` brings the count to 0, take the skip out (give the code an answer key) and write the expected file again.

Measured on the head (one finding in every row; the count is `other_sources_count`, the list in the report is cut at 5):

| handlers N | summary_elems | largest summary | reanalyses | other_sources_count |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 5,523 | 7 | 3,185 | 0 |
| 10 | 9,456 | 16 | 3,203 | 9 |
| 40 | 22,173 | 43 | 3,263 | 39 |
| 160 | 74,613 | 163 | 3,504 | 159 |
| 320 | 144,533 | 323 | 3,824 | 319 |

Both numbers are **linear in N**: `summary_elems` about 440 per handler (the 250 callers of `Decode` and their callers each
carry the N requests), and `other_sources_count` is exactly N-1. The acceptance of the second stage of
`docs/DESIGN-shared-state.md` is that `other_sources_count` becomes 0 and the finding stays the same.

## `longlived`: false "other sources" through a long-lived receiver that is passed whole

`go run ./bench/go-scale/gen -shapes longlived -ops N -out DIR` (by name only; not part of `-shapes all`). The shape of
`bench/gcp` that `bench/explore/LOG.md`, measurement 5, found: the request goes into a **long-lived receiver** (a
connection, a stream, the cache of a router), the receiver is passed whole to another function, which reads **another
field** and writes a package variable. N handlers (a source by its type, `*ll.Req`) each fill the one stream `Conn` by a
library call that fills it as a whole (`json.Unmarshal([]byte(r.Body), Conn)`; the engine has no model for what it fills);
`flush(s *Stream)` reads `s.Meta`, a field that has the tag `json:"-"` so the library never fills it, and writes the
package variable `Sent`. `Reader` reads `Sent` and sends it to the sink. Only handler 0 sets `Meta` itself, so the one
real finding is handler 0 to the sink; the requests of the other N-1 handlers never reach `Sent`, and the finding lists them
as `other_sources`. One in eight functions of the ballast calls `Last` (it flushes and returns `Sent`), so what `Sent` carries
goes up the call graph. `expected/longlived-w1.json` (N = 40): the one finding, `other_sources_count` 39, `summary_elems`
22,254; like `registry`, the profile skips `checkCuratedReport` (no answer key in the code).

Measured on the head (one finding in every row):

| handlers N | summary_elems | largest summary | reanalyses | other_sources_count |
| ---: | ---: | ---: | ---: | ---: |
| 10 | 9,477 | 16 | 3,207 | 9 |
| 40 | 22,254 | 43 | 3,267 | 39 |
| 160 | 74,934 | 163 | 3,508 | 159 |

Both numbers are linear in N (about 440 elements per handler, `other_sources_count` exactly N-1), as in `registry`.

**What it is the acceptance of.** `longlived` is a candidate for the acceptance of **stage 5** (a path in `flows` with
`from:` / `to:`, that tells the fields apart, the other field staying quiet): then `other_sources_count` becomes 0 and the
finding stays. `registry` stays the acceptance of **stage 3** (constant keys). `bench/gcp` stays a measurement and is not an
acceptance of either.
