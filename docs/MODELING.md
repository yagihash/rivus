# Writing models for rivus

A guide for whoever writes a model (a rule file that says what a function without a body does with taint), by hand or with
an AI. The sections below cover `pure`, `approximate`, `calls`, `parts`, `named`, and the rule that a row has to be hit.
Not written yet: the basic shapes of `flows`, `sources`, `sinks` and `sanitizers`; `globals` with `holds` (ADR-056);
`.content` (ADR-082); `read_impls`, `self_first` and `alias`. Until they are, read the built-in rule sets in
`internal/model/builtin` and the loader in `internal/model/model.go` (it rejects what it does not know), and the
examples in `examples/rules`. The principles behind the models are in [PRINCIPLES.md](PRINCIPLES.md) (section 3).

## 0. A row replaces what the analysis does by itself

A call that has no model is read this way: the receiver and the arguments flow to the result, and **the memory that the arguments point to is read as unknown** (a function that is not read may write into what it is given: `unmodeled_library_call`). **A row for the function replaces all of that with what the row says.** So a row that says only `flows: [{from: args, to: result}]` takes away the other two: the receiver no longer flows to the result, and what the function writes into its arguments is no longer there. A flow can be lost by adding a row, and nothing says so. (Found on `bench/gcp`: the first stub of `rivus models coverage` left the receiver out and lost 3 of 27 findings; the second left the writes into the arguments out and lost 2 more. The stub says the broad thing for the result and the writes, one row per argument that can be written into (see below). `TestModelsCoverageStubsLoseNoFinding`.) When you narrow a row, check what you took away: the receiver, each argument, the writes into the arguments, the callbacks.

### `result` is the first result only; every result is `results`

`to: result` (the same as `result0`) is **the first result only**. A function that returns `(value, error)` and has the row
`{from: args, to: result}` does not carry its arguments into the `error`: `err.Error()` of it reaches no sink, and nothing says so
(`strconv`-like functions that put the input in the error are the case that matters). Write `to: results` for every result, or one
row per result (`result0`, `result1`) when they differ. `result` is right only for a function with one result (and for Rust and Dart,
where a call has one result: `Result<T, E>`, a tuple and a Record are each one value). `rivus models coverage -yaml` writes `results`.
`TestGoRowsThatSayResultAreForOneResult` reads the signature of every Go function that has a `to: result` row in the built-in rule
sets, and fails for one with more than one result unless `testdata/result-first-only.txt` says, with the reason, that the other
results never hold the data (the error is a fixed text). In a model made from types (a stub for every function of a package), 19 of
1,475 rows had dropped such a flow, and `to: results` was the fix.

### A write into `args` is wide: name the argument

`{from: args, to: args}` and `{from: recv, to: args}` write the taint into **every** argument. An argument that is a shared
object (a mux, a client, a configuration) then holds the taint of every call that passed through it, and the findings that
follow from it grow (one project: about 8 times as many). Name the destination (`to: arg1`: the buffer, the struct that is
filled) and give the source as narrowly (`from: arg0`).

`rivus models coverage -yaml` writes the rows that way, and keeps the writes **on**: a row replaces the guess that a function
may write into what it is given, so a stub without the writes would silently lose a flow, and a finding that is lost is worse
than one that is added (the added ones show in the count). It writes one group of lines per argument that can be written into,
from the type of the operand at the calls: a pointer, a slice, a map, an interface (not a string, a number, a bool, a struct
held by value, a function), and one group for the receiver when it is a pointer. Each group has a comment: delete the lines
when the function does not write there. A `context.Context` is left out of the writes (it stays a source of the results: a
context made from a request is how that taint reaches what a client returns, and leaving it out lost 3 of the 27 findings of
`bench/gcp`). `TestModelsCoverageStubsLoseNoFinding` checks that every stub, as it is written, loses no finding of `bench/gcp`
and `bench/go-curated`. When the types are not known (a coverage index of an older version), the stub has the wide row
and says so.

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

### Stopping at a number, and a value of a narrow shape: `non-payload`

A parser of a number or a time puts its input in the error and **not** in the value (`numeric_stop`, above): a number cannot carry an injection, and
tainting it would report every query or template that takes it. The same holds for a value that is not a number but has a fixed narrow
shape: a bool, a value close to an enumeration, a short name of a fixed set of characters. `time.Parse` is the case: the only text of the value that
the `Time` keeps is the zone name (3 to 5 capital letters, or `GMT+n`; `parseTimeZone` in `time/format.go`).

When `make model-diff` shows a MISS for such a flow (the body has it, the model leaves it out on purpose), the row is not changed. The reason is recorded as a
`miss-reviewed` note of the kind `non-payload` (the three kinds are in `docs/MODEL-RUNBOOK.md`, section 9):

```yaml
# model-diff: miss-reviewed time.Parse@arg1 non-payload the zone name that the Time keeps: 3 to 5 capital letters or GMT+n; src=time/format.go:parseTimeZone
```

It needs **what the value is narrowed to** and **where the source shows it** (`src=<file>.go:<function>`); the tool fails without either. The flow of the body has to
be real (a flow that the reading of the bodies only guesses is `body-default`), and the narrow shape has to be certain from the source, not from a guess.
A session that builds models does not add this note: it writes the reason in the draft and leaves the decision to the advisor.

## 2. When a flow is a guess: `approximate`

If a flow is real in some cases and not in others (the output of an external program may or may not carry its
arguments), do not cut it and do not write it as a plain flow: write it with a reason.

```yaml
- symbol: (*os/exec.Cmd).Output
  flows: [{from: recv, to: result, approximate: "the output of an external program: whether it carries the arguments depends on the program"}]
```

The flow is kept. A finding whose path goes through it gets one uncertainty of kind `model_approximation`
(`attributable_to: rivus`, `detail` is the reason), so its confidence is never `no_uncertainty_on_path`
([ADR-067](adr/067-model-approximate-flag.md)).

## 3. A function that calls a function it is given: `calls`

`flows` says where the data goes. It cannot say that the function **calls** an argument (`Once.Do(f)`,
`strings.Map(f, s)`, `Iterable.map(f)`, `Option::map(f)`, `thread::spawn(f)`): what the callback does with what the library
hands it, and what it returns, is in the callback, which is code of the project. A row of `calls` says which argument is
called, with what, and where its result goes; rivus then reads the callback at the call, with the values it captured:

```yaml
- symbol: strings.Map
  calls:
    - fn: arg0              # the argument that is called (recv or argN; the receiver is not counted in argN)
      with: [arg1]          # what it is called with, parameter by parameter (recv, argN): the first entry is the first parameter of the callback, so a callback of (ctx, msg) needs [arg0, recv], not [recv]. Nothing: nothing flows in
      ret: result0          # where what it returns goes (result, resultN, recv, argN). Nothing: it is dropped
      throw: result1        # where what it throws goes (the same). Nothing: it is thrown out of the caller
      writes: {0: arg2}     # where what it writes through its parameter 0 goes, besides the value it was given
      mode: call            # call (default); go and defer: it runs on its own, what it returns is not read
```

* **`with` fills the parameters of the callback from the first, in order.** For a callback of `func(ctx, m)`, `with: [recv]` puts
  the receiver in `ctx` and leaves `m` with nothing, and the flow you meant is silently lost (the usual mistake of a model written
  with an AI). To fill a later parameter, write the entries by number: `with: [{param: 1, from: recv}]` (`from: none` leaves a
  parameter empty on purpose; write every entry one way, not both). The load **warns** (stderr, `rivus: warning: ...`) of a `with` whose
  number of entries is not the number of parameters of the callback, when it can tell that number: the row says it with `params: 2`,
  or the function is one of the standard library of Go that the built-in rows already know (`sort.Slice`, `strings.Map`, ...). Where
  it cannot tell, nothing is said: write `params` (or the named form) in a row for a callback that has more than one parameter.

  ```yaml
  - symbol: example.com/lib.Subscribe       # func Subscribe(handler func(ctx context.Context, m *Message))
    calls: [{fn: arg0, params: 2, with: [{param: 1, from: recv}]}]   # m is the receiver; ctx gets nothing
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
test too. When you write a row for a function of the **standard library**, write the call in the harness first, and read what it gives.

**A row of a package: `checked_by: cell`.** A harness runs the code it calls, and the code of a package is a third party's: it is not
run. A row of `calls` of a package (not the standard library: `dart:`, `std::`, a Go path with no dot in its first element) says
`checked_by: cell` on each of its rows, instead of being in a harness:

```yaml
- symbol: package:dio/src/interceptor.dart#Interceptor.onRequest      # a package
  calls: [{fn: arg0, with: [recv], checked_by: cell}]
```

`TestCallsRowsAreHeldByCells` (`cmd/rivus`) then wants a **cell** of the zoo (a file under a `cells` directory of `bench/zoo-*`) that calls
the function of the row (it is read from the IR, as `TestBuiltinRowsAreHitByTheZoo` does) and that has a `@sink(<id>)` (not `none`) or
a `@miss(...)` mark. The marks are held by the zoo tests of the language, at the level of the analysis: the flow that goes through the
callback is found, or is a miss with its reason. This is a weaker check than the run (it is the model compared with the analysis, not
with what the library does), so read the library's source for the row, and write the cell so that the flow can only go through the
callback. `checked_by: cell` on a row of the standard library fails the test (a harness can call it), and so does a harness that calls a
row that says it. All the rows of one function say it, or none does. Without it, nothing changes.

**A provided method that calls a method of its own trait: `method`.** `Write::write_all` is a method of the library that has a body
there (it calls `write` of the receiver in a loop). When the receiver is a type of the project that implements `Write`, what
`write_all` does is what its `write` does, and the library's body is not read. A row with `method` says so, instead of `fn`:

```yaml
- symbol: std::io::Write::write_all
  flows: [{from: arg1, to: arg0}]
  calls: [{method: write, with: [arg1]}]   # the receiver (arg0) is given as it is; `with` is what comes after it
```

The implementations of that method that the program has (its own code: `<T as std::io::Write>::write`) are called with the
receiver and `with`, as a call of them is. When the receiver is seen to be made here, of one type, only that type's is called.
**The positions of a row with `method` count every operand of the call, the receiver first (`arg0`)**, as the models of Rust do,
for a call of the function itself and for a call through a trait alike. `ret`, `throw`, `writes` and `mode` are not for it: the call
is a call, and its result, what it throws and what it writes are those of the method. The flows of the row are applied beside it
(the library's own half). The harness (`bench/zoo-rust/callbacks`) calls the real function with a type that records what its
`write` was given.

Not yet: a projection (`recv.elem`, `result0.err`: whole values only), a result that is itself a function (`sync.OnceValue`), a callback
that is a field of an argument, `feed` (the accumulator of `fold`), and the libraries that are not the standard one
have no harness (`checked_by: cell`, above).

A sink is found where the **project's own code** calls it: rivus lists sources and sinks in the functions of the project, not inside the
bodies of dependencies (ADR-099; the bodies are read to follow flows). A flow from the user's input to a dangerous call inside a
library is found only when a sink row stands at the library's API that the project calls. So a sink model is needed for detection, not
only to cut the cost of the analysis.

## 4. A sink that is one field of an argument: `parts`

A sink says which **position** is dangerous (`args`, `recv`). A request of a generated API is one value with a dangerous field and
safe ones (`spanner.Statement{SQL, Params}`: the SQL is dangerous, the bind parameters are not; a Cloud Tasks request: the `Url`,
not the `Body`). A sink that takes the whole value reports a flow that goes only into the safe field. Say which field:

```yaml
- symbol: (*cloud.google.com/go/spanner.ReadOnlyTransaction).Query
  args: [1]
  field: SQL                                  # one position: the short form
  category: sql_injection

- symbol: (*cloud.google.com/go/cloudtasks/apiv2.Client).CreateTask
  category: ssrf
  parts:                                      # several positions, or several fields
    - {arg: 1, field: Task.MessageType.HttpRequest.Url}
    - {arg: 1, field: Task.MessageType.HttpRequest.Headers}
    - {recv: true, field: Config.Endpoint}
```

* **`field` is the names of the fields from the value of the position down to the dangerous one**, separated by dots, as the IR of
  the language names them (Go: the Go field name, an embedded struct by its type name; Dart: `url`; Rust: the field, or `0`). The first
  name is a field **of** the value (not the type of it). **Do not write the steps of a pointer** (`*`): a pointer behind a field, or
  inside an interface (a `oneof`, an `any(&x)`), is skipped when the path is matched. `[]` is an element of a slice, an array or a
  map: `[].SQL` is the `SQL` of each element (it can be left out: an element is skipped too, and the elements are not told apart).
  Matching is by the names **in order**: a name of the path is never skipped.
* **A position in `args` or `recv` is dangerous whole**, and `parts` do not limit it. A sink with `parts` and no `args` reads only the
  positions of its parts (no `args` is **not** every argument, as it is for a sink without `parts`). A sink without `parts` is as it
  was. Several rows of one symbol are added up (rows of the same category are one sink; rows of different categories are a sink each,
  see 4a-1).
* **What the finding says.** Taint that is in the field is a finding as any other. Taint in a field **beside** it (`Params`) is not a
  flow to the sink, and the sink is quiet. When the value does not tell which field the taint is in, the finding is **kept and carries
  `field_imprecision`**, with the `why`: `whole_value` (the whole value is tainted: a function that is modelled to return the request
  tainted, a `json.Unmarshal` into it), `path_limit` (the path is longer than the analysis keeps: 3 steps, and each pointer is a step),
  `element` (an element of a container), `opaque_callee` (the value went to a function that is neither read nor modelled: it may have
  moved the taint into the field). A flow that is told to be in the field wins over one that is not: when both reach the sink from the same source, there is no
  `field_imprecision`.
* **A misspelled path is silent for ever, and looks like a decoy.** `Task.MessageType.HttpRequst.Url`, or a field that a new version of
  the library renamed, matches nothing: nothing reaches it, and the sink is quiet. Three guards: (1) rivus checks the names against the field names that the
  instructions of the analysed program use; a name that is nowhere makes the sink read the **whole** value, with
  `field_unresolved` (read the output for it: it is on the program, and on the findings of the sink). (2) Write **two tests** for
  every row, as for a `pure` row: a flow into the field (it must be found, **with no `field_imprecision`**), and a flow into a field beside it (it must be quiet): a
  sink that has only the second is not tested. (3) In a project, run the sink once on a case that puts a marker in the field and look for the
  finding, and for the absence of `field_unresolved`.
  The zoo does (2) for its own rows: `TestZooFieldSinkRowsAreHit` (`cmd/rivus/zoo_fieldsinks_test.go`) fails for a row with `parts`
  that no flow of the cells meets through the field itself.
* **A library whose fields are not exposed** (the Go protobuf Opaque API, builders, Dart setters, Rust builders) writes the field by a
  call (`req.SetUrl(x)`, `b.url(x)`): the call is a `flows` row, and until `flows` can name a field of its target (`to: arg0.Url`, not
  yet), the request is tainted as a whole and the sink reports it with `field_imprecision` (`whole_value`). That is
  safe, and noisy: the model of such a library is not finished by `parts` alone.
* The calls through an interface of the project whose implementation has the row (ADR-068) use the same `parts`.


### A worked example: Cloud Spanner and BigQuery (`internal/model/builtin/go-gcp.yaml`)

The first built-in rows with `parts` (ADR-077). The shape of each is a statement or a query that is one value, with the text in one
field:

```yaml
- symbol: (*cloud.google.com/go/spanner.txReadOnly).Query     # (1) the key is the type that declares the method
  args: [1]
  field: SQL                                                  # the text; the Params are not a sink
  category: sql_injection
- symbol: (*cloud.google.com/go/spanner.ReadWriteTransaction).BatchUpdate
  args: [1]
  field: "[].SQL"                                             # (2) the SQL of each statement of the slice
  category: sql_injection
- symbol: (*cloud.google.com/go/bigquery.Query).Run
  recv: true
  field: QueryConfig.Q                                        # (3) an embedded struct is a step, by its type name
  category: sql_injection
```

1. **Write the symbol that the IR names, not the one the documentation names.** `client.Single().Query(ctx, stmt)` and
   `tx.Query(ctx, stmt)` of a `ReadWriteTransaction` are both a call of `(*spanner.txReadOnly).Query`: the method is declared on
   an unexported type that the public ones embed. Read the symbols off the IR of a small project that calls the API
   (`rivus debug extract`, and look at `callee.symbol`), and do not guess them. A row whose key matches nothing is quiet for ever.
2. **`[]` is the elements.** A finding through an element carries `container_element_imprecision`: the elements are not told apart.
3. **Do not write the safe fields.** `Params`, `Parameters`, `Labels` are not a sink: a row names only the dangerous field, and the
   others are quiet because nothing reads them.
4. **A function of the library that builds the value** (`spanner.NewStatement(sql)`) needs no row when the library's own code is
   read: the text goes into `SQL` through the body. A library that is not read (a generated client, a builder) needs a `flows` row,
   and until `flows` can name a field (`to: result.SQL`), the value is tainted as a whole and reported with `field_imprecision`.
5. **Test each row three ways** (`bench/gcp`, `TestGCPRows`): a vulnerable case that must be reported with no `field_imprecision`
   and no `field_unresolved`; the fixed form (the value goes in by the bind parameters) that must be quiet; a decoy (the request is
   in another field of the same value) that must be quiet. The test also fails for a row that no case reaches.


## 4a-1. A sink that is a named argument, and a symbol with several categories: `named`

Dart's API takes its input by name (`Query(kind: x)`, `bucket.list(prefix: x)`). The extractor puts the named parameters after the
positional ones **by name**, and a call that leaves one out holds nil there, so the position of `prefix` is that of the declaration
(it moves when an override adds a parameter) and `args` cannot say it. Say the name:

```yaml
- symbol: package:gcloud/src/datastore_impl.dart#Query.new
  named: [kind]                               # the argument `kind:` of the call, wherever it is
  category: query
- symbol: package:gcloud/storage.dart#Bucket.list
  named: [prefix]
  category: object_name
- symbol: package:googleapis/pubsub/v1.dart#ProjectsSubscriptionsResource.create
  parts:
    - {arg: 0, field: pushConfig.pushEndpoint}
    - {named: request, field: pushConfig.pushEndpoint}   # `named` in place of `arg`, in parts
  category: ssrf
- symbol: package:googleapis/pubsub/v1.dart#ProjectsSubscriptionsResource.create
  args: [1]
  category: resource_name                     # the same symbol, another category: a sink of its own
```

* `named` is a list of names (a part has one, `named:`), beside `args` and `recv`: a position by name. The short form (`named: [x]`
  with `field:`) is one part. It is read from the call: the IR gives each operand of a call the name of the named parameter it is
  for (`arg_names`, parallel to `args`; empty for a positional one and for the receiver), so it works for a function that has no
  body (a dependency that is not read, a model).
* **A call that does not pass the name has no such argument**, and the sink is not a site there (a sink with only `named` is not
  every argument, as one without `args` is). Positions of `args` beside it are read as they are.
* **A misspelt name is not silent.** A name that no call of the symbol in the program has makes the sink read the **whole** call,
  and the analysis records `named_unresolved` (a misspelling, or a parameter that was renamed: read the output for it, as for
  `field_unresolved`). A name that other calls pass is only absent from this one. The names a call carries are those of the
  parameters of the callee as the extractor resolved it: a call whose callee did not resolve has none (a dynamic call through an
  interface carries the names of the interface method). Write the two tests of 4 for a row with `named`: the name given (found), and
  another argument given (quiet). `TestDartNamedArgumentSinks` (`cmd/rivus/dart_named_test.go`) is the shape.
* Go and Rust have no named arguments: `arg_names` is empty, and a `named` row there is `named_unresolved` at a call of the symbol
  (read whole): do not write one in a model of Go or Rust.
* **An unresolved name reads the whole call, on purpose**: the sink errs on the side of a finding and not of a missed one (the warning is `named_unresolved`, not a silence). `named` in `flows` and `sources` is #145.
* **A symbol with rows of several categories** has a site, and so a finding, for each category (one call is then several sites,
  and findings from the same source can come out of it, one per category). Before, the rows were one site that was reported under
  the category of the first row. **A row that reads the same positions as an earlier row of another category is not a sink of
  its own** (the same `args`, `recv`, `named`, `parts`): a project row for a symbol that a built-in row already names is one
  sink, under the first category, as it was; the finding lists all the categories in `categories` (only when there are two or more). `named` is for `sinks` only: `flows` and `sources` still name positions (a flow that takes a named
  argument is written by position: the declared one, until `flows` can say `named`).

## 4a. Cutting a dependency with a model (not reading its body)

### What a model does and does not skip today

A row with a sanitizer or a propagator makes `HasModel` true for the symbol (`internal/model/model.go`). At a call to that symbol the model is applied and the summary of the body is not used (`internal/taint/calls.go`). That is all it skips:

- The summary of the body is still computed: the function is in the IR and is analysed like any other. Nothing about the model makes the analysis skip it.
- A row that is only a source or only a sink is not a model: `HasModel` is false and the body is used.
- A function of the same package without a row is read as before. A package is not cut because one function of it has a model.
- Sites (sources and sinks) are listed only in the functions of the project, never inside a dependency (ADR-099; the project is the packages of the same module and the path packages under `-allow-deps-under`, ADR-114; `summary.dependency_sites` says so whenever a body of a dependency was read). A dangerous operation that the library does inside itself is found only when the call into the library has a sink row. So a model is needed for detection as well as for cost.

### When a flow model may stand in for a body

A model that stands in for the body of a dependency (so that the body is not read) is allowed when the body does not exist, when reading it does not converge, when the library calls the user's code (`calls:`), or when reading it costs the analysis a great deal (ADR-092). The last one has two criteria, and a package or a method has to meet one of them.

1. **A whole package** (ADR-092): all of these: (a) the cost is measured and written down with the target it was measured on (a rule of thumb: the dependency is 90% of the time, or gigabytes); (b) a check compares the result with the body read and the result with the model, on a fixture that uses the API: the set of flows and the **edges at the border** (which function of your code a call into the dependency calls back) have to be the same, and the model may only be the side that has more, with a note; (c) every API that calls a function it is given or holds has a `calls:` row (a form that the format cannot write is a problem of the format, and the call site says so, not silence).
2. **One method of an interface with many implementations** (ADR-098): when the implementations delegate to the same method (a clique of delegation, such as `Client.send` of `package:http`), the row of that method is a `flows` model and the bodies of the implementations in dependencies are not read. Two conditions: (a) on public targets the set of findings is the same as with the bodies read, and the targets used are written in the design document; (b) the bodies that were not read are marked with a note of kind `dependency_body_unread`, not silently. The model is only for that method. A thin convenience method that calls it (`get`, `post`) has no model, only a sink if it needs one, and its body is read (cutting it would remove the edge from your code to the call of `send`). The implementations in your own code are read as before. The limit: a function passed to a constructor and called later (`RetryClient(when:)`) cannot be written with `calls:` yet, so only the note appears.

A function of a cut package without a row is the unmodelled call it is (its arguments flow to its result, with `unmodeled_library_call`).

### Cutting by package: `body_replaced_packages`

A rule file (a built-in one, or yours) can declare that its models stand for the code of a package of the dependencies, and that the analysis does not read it:

```yaml
body_replaced_packages:
  - package: github.com/aws/aws-sdk-go-v2/service/s3/...
    reason: the rows of s3.yaml were compared with the body (MISS 0); the callbacks are in calls: rows
  - package: package:googleapis
    reason: ...
```

- `package` is the unit of the functions, the one `summary` and the notes use: the import path of a Go package, `package:<name>` of a Dart package, the name of a Rust crate. **It names that package and no other**: `example.com/m/a` does not take `example.com/m/a/b`. `example.com/m/a/...` (the ending `/...`, as the go tool writes it) names the package and everything under it. `reason` is required: what was checked.
- Only the functions of **dependencies** are left out. A package of the project is always read (a declaration for one has no effect, and `-body-replaced` says so). The standard library is not read anyway.
- A declared package is not read by the analysis: its functions have no body in the program. A call of one of its functions is **the model when the model has a row for it, and `unmodeled_library_call` when it has none** (what `-no-deps` does, for this package alone). The note says that the package is declared, so the call is not silent. `-no-deps` makes a declaration pointless.
- **Callbacks into your code.** A package that is not read cannot be seen calling a function it was given. Where the model has no `calls:` row for the function, rivus does not drop the callback: a function value given to the call is taken to be called with what the other operands hold, and its result is the result of the call; the methods (up to 8) of a value of a type of the project that is given as an interface (Go) are taken to be called the same way. Each is an `assumed_call` note. This is the safe default, and it is a guess: write the `calls:` row (section 3) to say what the function really does, and no note is made. A value that is made an interface somewhere else than at the call (a field, a parameter) is not seen: the check before a declaration is yours.
- **A function that has no body and no model, given a function.** The same default holds for a function of the standard library or of a dependency that is not read and has no row at all (ADR-116): a closure that is given to it, and that the program has the body of, is taken to be called. Any of the other operands (and the receiver) may reach any of its parameters (a union: what the library hands it is not known; a number or a bool carries nothing), and what it returns is taken to reach the result of the call, when there is one. A call without a result is covered the same way. It is an `assumed_call` that names the function. A function that has a row (even one without `calls:`) or a body is not changed by this; write the row (section 3) to say what the function really does.
- **The extractor does not read a declared package either** (ADR-105). The analysis of the project and the findings are the same as when the extractor reads it and the core leaves its functions out (`-drop-after-read` runs that, to compare). Go: the syntax and the type information of the package are let go of before `go/ssa` is built (the type check of every package that `go/packages` loads is still done). Dart: the files of the package are not resolved or lowered (the library is still built for its declarations and types, and the packages it imports are still found). Rust: the driver of the extractor lowers none of the bodies of a declared crate (`--body-replaced <crate>`; `actix-web` and `actix_web` name the same crate), and says how many functions it has; `cargo check` still compiles the crate, so the time and the memory of the extractor do not fall: what falls is the size of the IR and what comes after it. What is read less is in `docs/DESIGN-package-models.md` (section 7a). `summary.body_replaced_packages[]` then has `functions_declared` (what the package declares) and no `functions` (nothing was read, so nothing was left out). A package that only a declared package uses is read as before, and does not reach the analysis (it is not in the program when nothing of the project reaches it).
- The summary says what was left out: `summary.body_replaced_packages`, a list sorted by package, with `package`, `declared_in` (the rule file, `builtin:<file>`, or `-body-replaced`), `reason` and `functions` (how many functions were read and then left out) or, for a package that the extractor was told not to read, `functions_declared` (how many functions the package declares; `functions` is then not there). A declaration that left out nothing is not listed.

For a try, or to compare, without editing a file:

| Flag | What it does |
| --- | --- |
| `-body-replaced PACKAGE` (repeatable) | declares the package for this run. The reason is "named on the command line". |
| `-read-declared-bodies` | reads the bodies of every declared package (the rule files' and the flag's): the run that a declaration is compared against. `summary.body_replaced_ignored` says how many were left out. |

**When to declare** is the condition of ADR-092 and ADR-098: the flows are compared with the body (no MISS; the extra flows only on purpose), and every API that calls a function it is given or holds has a `calls:` row. Compare with `-read-declared-bodies`: the findings have to be the same, and the differences are only on the functions that have no row (they are `unmodeled_library_call`, on purpose). The declarations are part of the key of an index (`rivus index`): an index made with other declarations is stale.

## 4b. Categories

A sink has a `category`: what kind of harm the value does there. It is a free string in the format, and these are the ones the built-in rows use:

| category | the value is |
| --- | --- |
| `command_injection` | a command or its arguments |
| `path_traversal` | a path of the file system |
| `sql_injection` | the text of a query (SQL, BigQuery, GQL). A value that the service binds (a parameter, a filter value) is not one |
| `ssrf` | a URL or a host that the program sends a request to |
| `xss` | text that is written into HTML |
| `open_redirect` | the address of a redirect |
| `resource_name` | the name of a resource of a service that the user decides: a bucket, an object, a topic, a subscription, the kind of a datastore key. It can point to another resource of the project, or of another project. It is not a path of the file system (`dart-gcp.yaml`, `docs/GCP-MODELS.md` section 8) |

`rivus analyze -categories a,b` reports the findings of these categories only, and `-skip-categories a,b` all but these (not both; a name that no loaded rule uses is an error that lists the valid names). The analysis is the same; the findings that were left out are counted in `summary.categories_excluded` (`category`, `findings`), which is absent when nothing was left out. A finding with several `categories` stays when one of them is reported, and is left out when all of them are. `index` and `graph` do not take the flags.

A new category is made when no other one fits (not for a new library). It is added to this table, and to the decision of the product (`docs/GCP-MODELS.md`, judgement E, for `resource_name`). One symbol has one category (#140).

## 4b-1. A sanitizer says which kind of sink it is safe for: `neutralises`

A sanitizer row drops a label (`drops`) and gives the value another (`adds`). A sanitizer is safe for **one kind of sink**, not for all: an HTML escape makes a value safe in HTML, and the same value is still a command, a path or a query for the sink that is one. `neutralises` names the categories (section 4b) of the sinks that the value is safe for. A sink of another category still reacts to the value, which now has the label of `adds`:

```yaml
sanitizers:
  - symbol: html.EscapeString
    drops: [user_input]
    adds: [html_escaped]
    neutralises: [xss]          # SQL, a path, a command and a URL still see the value
```

- A sink accepts a label that a sanitizer gave unless its category is in `neutralises` (a symbol with several categories: unless all are), and only if it would have accepted the label that was dropped. The finding shows the label (`html_escaped`) and the sanitizer is a step of its path (`sanitized by ...`).
- `neutralises: []` is the claim "none": the value is only marked as having passed the sanitizer, and it reaches every sink. Write it when it depends on where the value lands (a percent-encoded value is no host, and still a path that can climb). A claim that hides a flow is made only where it holds in every place the sink can read the value; a claim that is only sometimes true is a missed flow.
- `adds` is required with `neutralises`, and `drops: ["*"]` cannot have it (name the labels).
- A builtin row has to write `neutralises` (`TestSanitizersAreReviewed`), and an empty one has its reason in `testdata/sanitizers-reviewed.txt`.
- **A sanitizer without `neutralises` keeps the old meaning: the value is hidden from the sinks of every category** (the rules of a project written before this one go on working). rivus says so once on stderr (and `summary.project_rules[].sanitizers_without_neutralises` counts them).
- A sanitizer that is a method takes what it is called on as data as well as its arguments (what the row does not drop goes on to the result).
- A value that passed two scoped sanitizers has both labels (`url_encoded&html_escaped`) and is hidden from the union of their categories. Known limit: a sanitizer without `neutralises` in the chain keeps the old meaning for the value it receives. The label that `adds` names is one label for all the rows that give it; two rows with different `neutralises` for the same label keep only the categories that both have.

## 4c. Which versions the rows were checked against: `verified_versions`

A rule file (a built-in one, or yours) records the versions of the packages that its rows were checked against ([DESIGN-package-models.md](DESIGN-package-models.md), 6.7):

```yaml
verified_versions:
  - package: shelf_router        # Go: the module path; Dart: the pub package; Rust: the crate
    versions: ">=1.1.0 <2.0.0"   # what was checked: 1.1.5, v1.1.5, ^1.1.0, ~1.1, or comparators (blank or comma between)
    checked_with: 1.1.5          # optional: the one version that was read; inside `versions`
    checked: 2026-11-22          # the day, YYYY-MM-DD (not in the future)
```

- A record changes no row and no result. **For Go, the analysis compares it with the versions that the program uses** (`go.mod`, as `go list` says): a module that has a record, used at a version outside every range of its records, is one entry of `summary.unverified_versions` (`package`, `used`, `verified[]`, `source`), sorted by package, also when there is no finding. The rows are applied all the same. A module with no record is not listed (most built-in files have none, and a note on every run would say nothing); neither is one that a directory replaces (no version). A pre-release or `+incompatible` suffix of the used version is not read. **Dart** is compared the same way, with `pubspec.lock` in `-C` (the hosted packages; a path or git dependency has no version and is not listed; `source` is `pubspec.lock`; a pre-release is read by its numbers; a monorepo or workspace whose lock file is not in `-C` is not supported). **Rust** the same, with `Cargo.lock` in `-C` (crates of a registry; a path or git crate is not listed; `-` and `_` in a crate name are the same; a crate that the lock has at several versions is one entry when any of them is outside the ranges, `used` is the first one outside and `all_used` lists them; `source` is `Cargo.lock`; a lock file in a workspace root above `-C` is not read). A record is compared with the dependencies of its own language only: the built-in files `go-*`, `dart-*` and `rust-*` (or a rule set named `rivus/<language>/...`) say it, so a Dart record of `http` is not compared with the crate `http`. A rule file with no such name (your own) is compared with the dependencies of every language, so name a package that exists in one language only, or use a name that the others do not use.
- The file is refused when `package` is empty or a pattern (`/...`), `versions` is empty or not a version or a comparator, `checked` is missing or not a day, `checked_with` is not one version or is outside `versions`, a key is unknown, or a package is recorded twice in the file. The same package in two files is kept as two records. It is not checked that a row names the package.
- `rivus models versions [-rules FILE] [-no-builtin-rules]` lists the records (JSON: `package`, `versions`, `checked_with`, `checked`, `source`).

## 4c-1. Candidates that were looked at and not adopted: `rejected:`

A model file records the candidates that were looked at and left out, in the same file as the rows ([DESIGN-package-models.md](DESIGN-package-models.md), 6.11):

```yaml
rejected:
  - symbol: github.com/jmoiron/sqlx.LoadFile   # as in the rows
    kind: sink                                  # sink | source | sanitizer | calls
    position: arg1                              # optional: the place that was looked at (arg0, recv, result, field, arg1.Field; Dart: named:<name> for a named parameter)
    why: "the argument is the path of a file, not a query"
```

- A record, nothing more: the analysis does not read it, and it changes no row. It replaces the comment "not modelled, on purpose" at the top of a file, as data.
- The file is refused when `symbol`, `kind` or `why` is missing (or blank), `kind` is not one of the four, a key is unknown, or the same symbol, kind and position are written twice. `rivus models` and every other reader of a rule file take the key.
- `make model PKG=<Go package | Dart package> VERSION=<v> LANG=<go|dart>` (`bench/model-author`) lists the candidates of layer 1 of a package (the rule table is `bench/model-author/rules.go`) and matches each with a row or with an entry here. A candidate with neither is undecided: the command exits with 70 and writes the list in `.claude/work/model-author/` (a work file, not committed). An entry whose symbol is not in the package any more is a warning.
- A package whose bodies are read (not the standard library, and not in `body_replaced_packages`) has no `calls` candidates: the analysis resolves the call of a function value or an interface, and a row of `calls:` is for a body that is not read. The methods of exported interfaces are candidates like the others.
- What is checked is the majors that upstream still supports (decided by public facts; one line of the evidence at the top of the model file or in the pull request). Run the command once per major with `VERSION`; a row or an entry written with `/v*` is read for the major that is run. `verified_versions` keeps one record per major with the concrete module path (`x/v2`, `x/v3`): the loader refuses the same `package` twice.
- An entry with no `position` covers every place of the candidate; with `position`, only that place.
- A **type entry** rejects, in one entry, the places of a kind that are in an argument of one type: `type:` (the full name of one type: Go `pkg/path.Type`, Dart `package:x/src/a.dart#Type`; no pattern, no list; one of `symbol` and `type`), `kind`, `why` (say where the value of that type is checked, as a fact), and `covers: N` (required). `make model` counts at every run how many candidates (the ones with no row) each type entry covers, writes the counts in the summary of the pull request, and **warns when the count is more than `covers:`**: a new API entered the rejection unseen. Whoever sees the warning looks at the new candidates, then updates `covers:`. The places are the ones of the rule table that are fields of an argument of the type (`arg0.owner`) or an argument that is an interface of the package (`calls`). It needs no `position`.
- `deferred:` is for a candidate that could not be decided yet: `symbol`, `kind`, `why`, `position` (optional) and `ref`, which is required: `#N` (`docs/issues/N-*.md`) or `model-<package>-<name>` (`docs/issues/model-<package>-<name>.md`). The loader checks the form, a test (`TestDeferredRefsExist`) and `make model` check that the file exists. A record only, as `rejected:`. `make model` does not count it as undecided, and lists the entries (count and list) in the work file for the pull request. A place can be covered by `rejected:` and another place of the same candidate by `deferred:`. The summary of `rejected:` (how many entries this change adds, and the reasons by group) is written to `.claude/work/model-author/<package>-rejected-summary.txt`; `-breakdown` writes the candidates by kind, rule, word and type shape.

## 4c-2. Bulk adoption: `adopt_rules:`

A model file can record a decision for a rule of layer 1 and a scope, once, instead of one row per candidate ([DESIGN-package-models.md](DESIGN-package-models.md), 6.12; ADR-113). Step 1 is the format and the loader; step 2 (Go) is the command that expands a decision into rows (below).

```yaml
adopt_rules:
  - id: owner-repo-of-services            # unique in the file; a generated row names it in `decision:`
    kind: sink                            # sink | source
    rule: sink.arg-word                   # the id of a rule of layer 1 (`bench/model-author/rules.go`; never renamed)
    words: [owner, repo]                  # the words of the vocabulary of the rule that the decision takes
    category: resource_name               # sink only (a source has `labels: [user_input]`)
    scope:                                # no pattern over symbols
      package: github.com/google/go-github/v*/github   # exact (`/v*` is the major)
      receiver_suffix: Service            # or receiver_prefix, or receiver_types: [exact names]; functions: [exact names]
    why: "owner and repo are in the path of the request"
    covers: 56                            # how many rows it expands into now (required)
    representatives: ["(*github.com/google/go-github/v*/github.RepositoriesService).Get"]
    diff_checked: {version: v93.0.0, count: 56}   # optional until verified
    extra_ok: "the same shape as the hand rows"   # one reason for the EXTRA of the diff test
    extra_covers: 3                        # how many EXTRA it covered (required with extra_ok); the command warns when there are more
```

- `rule: sink.arg-type` selects the places by the type of the argument: `arg_types: [<full type names>]` (one type each) and a scope narrower than a package (`receiver_suffix`, ...). The other rules that take words say `words`; the rest have a fixed vocabulary and no `words`.
- A record, as `rejected:`: the analysis does not read it. The loader refuses a missing `why` or `covers`, an unknown rule id (or one of the other kind), a pattern in the scope, a key it does not know, and two decisions with the same id. The messages name the entry (`adopt_rules[1] (id) (line N)`). Whether a word is in the vocabulary of the rule is checked by the command, which has the table.
- A row that the command generates says `origin: generated` and `decision: <id>`, and is in `<lang>-<package>.adopt.gen.yaml` next to the hand file. `decision` is one id or, for a row that decisions that overlap were merged into, a list of all their ids (dictionary order, no duplicates: `decision: [a, b]`). It is only for `origin: generated`, and a generated row must have it. When the files are loaded together (`builtin.Load`, `model.CheckAdoption`), every id of `decision` must be in `adopt_rules:` of the hand file of the same package, with the kind of the row; a generated row elsewhere, or `adopt_rules:` in the generated file, is refused.
- Two decisions that claim the same place: the same category merges into one row with a warning (both decisions named; the row lists all of them in `decision:`); different categories stop the command (exit 70) for a human to decide. The detection is `ResolveClaims` in `bench/model-author`.

**Expanding (step 2, Go): `make model PKG=<pkg> VERSION=<v> LANG=go ARGS="-expand"`.**

- It reads the `adopt_rules:` of the hand file whose scope is the package, runs the rule of each decision (by its id, `words` or `arg_types`) over the fetched API (the candidates of `-recall`; each position carries the id of the rule that made it), keeps what the `scope` allows, and writes the rows to `internal/model/builtin/<hand file>.adopt.gen.yaml` (header: `generator`, `input`, `rules` = hash of the rule table, `decisions` = hash of what of the decisions changes the rows). Rows are sorted; the same input gives the same bytes. `-out <dir>` writes there instead (a trial; a work file with the lists is in `<work>/<package>-expansion.txt`). A fetch only: nothing of the package is run.
- A hand row wins: a position that a hand row has, or that `rejected:` or `deferred:` decides, is not generated. The output says, per decision, how many symbols it claims, how many have a hand row, how many are decided by `rejected:`/`deferred:`, and how many hand rows no decision covers. The rows of an earlier expansion (`*.adopt.gen.yaml`) are never taken for hand rows.
- `covers: N` is the number of symbols that the decision claims (before the hand rows and `rejected:` take their share, so adding a hand row does not change it). **More than `covers`: exit 70**, the new symbols are listed (a new API now matches: raise `covers` or narrow the decision or reject some), and the file is not written. Fewer: a warning. The number is written back to the model file only with `-write-covers` (it edits that one number; it never happens without the flag, and a growth that is accepted this way is read in the diff).
- Two decisions on the same position: the same category merges into one row (`decision: [a, b]`) with a warning; different categories stop with exit 70 (`ResolveClaims`), and nothing is written.
- Representatives: each decision names rows of its own in `representatives:`; one row per combination of positions (`arg1,arg2`, `arg0.Object`, `field`, `result`), the 3 combinations with most rows at most. A representative that is not a generated row of the decision stops with exit 70 (after the file is written); a combination with none is a warning that names a symbol to use. The zoo cell of a representative is a later change (the zoo test is not changed by this step), so a real `*.adopt.gen.yaml` is not committed yet: `TestBuiltinRowsAreHitByTheZoo` asks that every built-in row is hit.
- The words of a decision must be in the vocabulary of its rule (exit 2 if not); `sink.arg-type` needs a type of the package (`pkg/v*.Type`, a pointer or a slice is read through) and a scope narrower than the package. `extra_ok` and `extra_covers` are only recorded.
- Exit codes: 0 done; 2 usage or a decision that names a word outside the vocabulary; 10 fetch; 20 reading the package; 70 a conflict, a growth of `covers`, or a representative that is not a generated row. Dart and Rust are not expanded yet (exit 2).

## 4d. A source that is a parameter of a function that is given: `callback_param`

A router calls the handler that it is given with what it took from the request: `router.get('/item/<id>', (Request r, String id) => ...)`
hands the text of the path to `id`. The type of `id` is `String`, so `param_type` cannot name it, and `calls:` has no place for a source.
`kind: callback_param` says: the parameters `from` to `to` of the function that this call is given as its operand are a source.

```yaml
sources:
  - kind: callback_param
    symbol: package:shelf_router/src/router.dart#Router.get   # the call that is given the function
    operand: 1        # which operand of the call is the function (required; counted as the table below says)
    from: 1           # the first parameter of the function that is a source (0 is the first; default 0)
    # to: 2           # the last one, included (default: the last parameter the function declares)
    labels: [user_input]
```

* The function is found **where the call is made**: a function written there (a closure, a function literal), a function that is
  named there (a top-level function, a method value), or a local that holds one of them, also through copies and conversions. Then
  its parameters are a source, as those of a `kind: param` row are, and the flow in it is found by the analysis as any other.
* A function that is not found where the call is made (a parameter of the function that registers, a field, an element, a
  result of a call, a variable that two functions are stored in) is **not followed**, and the call says so: an `unresolved_callee`
  uncertainty at the call (in the function that makes it, which is the helper when a helper registers). The parameters of such a function are not
  a source: the flow is missed, with the note. The detail of the note starts with `callback_param_unresolved:` (so that it is not taken for any other
  `unresolved_callee`), and `summary.callback_param_unresolved` has `count` and `sites[]` (`func`, `position`, `symbol`: the call, not the function
  that would have been the source, which is not known), also when there is no finding at all. Write the row for the call that has the function literal (the helper's own callers are not followed).
* `from` and `to` are positions among the parameters that the function declares (a captured value is not one; a method
  value's receiver is not one). Named parameters are not counted: positions only (#145).
* **`operand` is counted the way the `calls:` rows of the same file count**, so that one model file does not have two ways:

| language | `operand: 0` is | the receiver |
| --- | --- | --- |
| Go | the first argument | not counted (`recv` in `calls:`) |
| Dart | the first argument | not counted (`fn: arg0` is the first argument after the receiver) |
| Rust (a set with `self_first: true`) | the receiver of a method, else the first argument (`self` is `arg0`) | counted: `arg0` |

* The row is read by the loader, with its keys (`operand`, `from`, `to`) only on this kind; an older rivus refuses the file
  (it does not drop the row quietly).
* The row cannot be checked against a body (the source is what the framework promises, not what the code of the function says):
  `bench/model-diff` compares the propagator rows only, so a `callback_param` row is not compared with the code of the library
  (a function that takes a function stays `unverified` there). The zoo is what holds it:
  `bench/zoo-dart/cells/lib/dart/callback_param_*.dart` run a router, and the rules of the zoo (`bench/zoo-dart/cells/rules.yaml`) have the row.
* `rivus models coverage -yaml` starts with a commented example of the row.

## 5. A row has to be hit

A row whose key is not the name the extractor writes never matches a call, and nothing says so: the call is read as an
unmodelled one (its arguments flow to its result, with an `unmodeled_library_call`), or, for a source, a sink or a
sanitizer, the call is not seen as one. The extractors do not always write the name you would expect: a call on a `List` in
Dart is the member of `Iterable` (`dart:core#Iterable.join`); a call on a `Vec` in Rust is the member of the slice
(`std::slice::sort`, `std::slice::iter`), a call on a `String` the member of `str`, a method of a trait the method of the
trait (`std::iter::Extend::extend`); a type that a library re-exports is written with the re-export (`tonic::Request`), and
serde writes its private module with its version in it. **Read the name from the IR** (`rivus debug extract` for Go;
`rivus-dart` and `rivus-rust` for the others), do not write it from the source.

`TestBuiltinRowsAreHitByTheZoo` (`cmd/rivus`) holds every row of the built-in model that names a function to the IR of
the zoo (`bench/zoo-*`) and the curated targets (`bench/*-curated`): a row has to be hit by a call (or, for an entry point,
by a function) in it, or be in the `.dead-allow` file next to the model file of the row (`internal/model/builtin/<model file>.dead-allow`, one line `symbol | reason`; the language is the one of the file name) with its reason. **A new row is a failing test until a cell
calls it**: add the call to a cell of the zoo, to a function of a curated target, or, for a package of the Dart team, to
`bench/zoo-dart/pure` (`lib/observe.dart` calls the rows of package:path with a marker and `TestZooPathDart` holds the
flows to the run; `lib/probe.dart` only has calls whose spelling is looked at). The IR of real projects is not used for
this: a real project does not use every API, and a row it does not use is not wrong. The `.dead-allow` file is for what
a run cannot check (an API that needs a server, a device, a network, or a library the zoo has no dependency on); the lines
marked `debt` are the rows of the standard library that no cell calls yet: they only go down, and none is added for a new
row. The test fails on a line for a row that does not exist, and on one that is in the file of another model file than the row's (so a pull request that adds a model file adds its `.dead-allow` file, and touches no shared list). A line for a row that is hit now is only logged: what hits it can be the code of a dependency of a cell, and a model pull request must not have to edit the `.dead-allow` of other models. The clean-up of such lines is a run of its own with `RIVUS_DEAD_ALLOW_STRICT=1`, where it fails.
