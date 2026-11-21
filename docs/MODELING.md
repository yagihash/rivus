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
is wrong. Of the 272 `pure` rows of Go (452 with Dart and Rust), **22 are called, 250 are not** (the list with the reason of
each is [`bench/zoo-go/pure/unchecked.txt`](../bench/zoo-go/pure/unchecked.txt): 52 have a string or bytes argument but act on a process, a file, the network or
the clock, or need a value the harness cannot make; 190 have no string or bytes argument, the receiver being the only way
in, which a call with a marker does not cover). Dart (`dart-pure.yaml`) and Rust (`rust-pure.yaml`) are checked in the zoo of
each language, by the same rule.

## 2. When a flow is a guess: `approximate`

If a flow is real in some cases and not in others (the output of an external program may or may not carry its
arguments), do not cut it and do not write it as a plain flow: write it with a reason.

```yaml
- symbol: (*os/exec.Cmd).Output
  flows: [{from: recv, to: result, approximate: "the output of an external program: whether it carries the arguments depends on the program"}]
```

The flow is kept. A finding whose path goes through it gets one uncertainty of kind `model_approximation`
(`attributable_to: rivus`, `detail` is the reason), so its confidence is never `no_uncertainty_on_path`
([ADR-067](DECISIONS.md)).

## 3. A function that calls a function it is given: `calls`

`flows` says where the data goes. It cannot say that the function **calls** an argument (`Once.Do(f)`,
`strings.Map(f, s)`, `Iterable.map(f)`, `Option::map(f)`, `thread::spawn(f)`): what the callback does with what the library
hands it, and what it returns, is in the callback, which is code of the project. A row of `calls` says which argument is
called, with what, and where its result goes; rivus then reads the callback at the call, with the values it captured:

```yaml
- symbol: strings.Map
  calls:
    - fn: arg0              # the argument that is called (recv or argN; the receiver is not counted in argN)
      with: [arg1]          # what it is called with, parameter by parameter (recv, argN). Nothing: nothing flows in
      ret: result0          # where what it returns goes (result, resultN, recv, argN). Nothing: it is dropped
      throw: result1        # where what it throws goes (the same). Nothing: it is thrown out of the caller
      writes: {0: arg2}     # where what it writes through its parameter 0 goes, besides the value it was given
      mode: call            # call (default); go and defer: it runs on its own, what it returns is not read
```

* **`with` and `ret` name whole values** (`recv` is the receiver with what is inside it), as `flows` does. For a Rust method the
  receiver is `arg0` and the closure `arg1` (the model of Rust names the arguments with `self` first).
* **A row that has a `ret` has what stands for it when the callback cannot be told** (it is a parameter, it was read from a
  field, there are more than four): `with` flows to `ret`, whole, and each use of this flow is an `assumed_call`
  (the function that was given is not known here: what it is called with is taken to come out as what it returns). The load
  refuses a row with a `ret` and no `with` unless `flows` says where the result comes from (`ok_or_else`: `flows` has it).
  A `flows` you write is added to this, not replaced by it: put there what `calls` cannot say (the other half of `Result::map`,
  `flows: [{from: arg0, to: result0}]` for the `Ok` that goes through `map_err`).
* **Where the callback is told** (a function, or a closure written right there: one to four of them), the flow of the row is
  not used: the summary of the callback is applied, with its parameters bound to `with`, what it captured bound, its result sent
  to `ret`, what it throws to `throw`. A finding that goes through it has no `assumed_call`, and a closure that is only given to
  such a call is not also applied where it is made.
* A function that returns nothing and carries nothing is `pure: true` with its `calls` (`sort.Slice`, `Once.Do`).

**The run decides the row** (`TestZooCallbacksGo`, `TestZooCallbacksDart`, `TestZooCallbacksRust`; `bench/zoo-go/callbacks`,
`bench/zoo-dart/callbacks`, `bench/zoo-rust/callbacks`): a harness calls the real function with a closure that records what it was
called with and returns a marker. A `with` that is not what the closure was given, a parameter that has the marker of an
argument the row does not name, a `ret` that is not in what the call returned, a `ret` that is missing and the marker is there, a
callback that was not called, are all errors of the row (corrected; not marked as misses). A row that no harness calls fails the
test too. When you write a row for a function of a library, write the call in the harness first, and read what it gives.

Not yet: a projection (`recv.elem`, `result0.err`: whole values only), a result that is itself a function (`sync.OnceValue`), a callback
that is a field of an argument, `feed` (the accumulator of `fold`), and the libraries that are not the standard one (a row for
`dio` or `tokio` is the next step; the harness takes them from a lockfile of their own).
