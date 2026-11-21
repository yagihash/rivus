# Writing models for rivus

A guide for whoever writes a model (a rule file that says what a function without a body does with taint), by hand or with
an AI. This is its first section; the others (the shape of `flows`, `sources`, `sinks`, `sanitizers`) will follow it.

## 1. When a function may be `pure`

`pure: true` in a built-in rule set says that **no taint flows through the function**, and removes both the default
(what goes in comes out) and the uncertainty of an unmodelled call. **A function may be `pure` only when none of
its results, and none of the places it writes to (a destination argument, the receiver), can hold the contents of an
argument or of the receiver. The `error` result counts: the text of an error often has the input in it
(`strconv.Atoi("x")` fails with `parsing "x"`, `os.Stat(path)` with the path, `time.Parse` with the value), and
that text reaches a response or a log.** In practice (R-03):

1. the results are numbers, bools, or nothing, and the error is nil or does not quote an argument (a lock, `Unix`, a
   predicate that does not fail with a message);
2. or the results do not depend on the arguments (`context.Background`, `os.Hostname`, a random key);
3. or the result is a handle or a value of the runtime whose contents are never read back as the text of an
   argument (a hash state, a logger), or a number-made value (`time.Time`, a formatted number).

Write the flows **per result** (`result0`, `result1`, `arg0`), not `pure` and not "args to everything": a parser of
a number or a time (`strconv.Atoi`, `ParseBool`, `ParseFloat`, `ParseUint`, `time.Parse`, `ParseInLocation`,
`ParseDuration`) puts its input in the error (`flows: [{from: arg0, to: result1}]`) and **not** in the number or the
time (the value is stopped by `numeric_stop`; tainting it would make false findings). A function that writes into a
destination says so (`hex.Encode`: `{from: arg1, to: arg0}`).

A function that returns a string, bytes, a slice, a map, a struct, a pointer or an interface, and takes the text it
returns from an argument or from its receiver (`exec.LookPath`, `(time.Time).Format` with its layout, `(*url.URL).RequestURI`,
`(*json.Decoder).Token`, `os.Stat`), gets `flows:`, not `pure`. `TestPureFunctionsThatReturnDataAreReviewed`
(`internal/model/builtin`) lists the `pure` functions of the standard library that return data and compares the list with
`internal/model/builtin/testdata/pure-carriers.txt`: **a function added to `pure` with such a result fails until it has been
read against this list and written in the file, with the reason**; one that got a `flows:` fails until it is taken out.
(Only Go's standard library is listed: Dart, Rust and the packages of other modules are not checked by the test; read them
by the same rule when one is added.) Known limit: a writer made by a `pure` constructor (`multipart.NewWriter(w)`) is not
linked to its destination: the flow into it does not reach `w`.

The run decides, not the reading: `TestZooPureGo` (`cmd/rivus/zoo_pure_test.go`, `bench/zoo-go/pure`) calls the `pure` rows
whose arguments are strings or bytes with a marker, in two variants, and a part (a result, or `error`) whose text differs
is wrong. Of the 269 `pure` rows of Go (449 with Dart and Rust), **23 are called, 246 are not** (the list with the reason of
each is [`bench/zoo-go/pure/unchecked.txt`](../bench/zoo-go/pure/unchecked.txt): 52 have a string or bytes argument but act on a process, a file, the network or
the clock, or need a value the harness cannot make; 194 have no string or bytes argument, the receiver being the only way
in, which a call with a marker does not cover). Dart (`dart-pure.yaml`) and Rust (`rust-pure.yaml`) are checked in the zoo of
each language, by the same rule.
