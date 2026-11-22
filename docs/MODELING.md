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
test too. When you write a row for a function of a library, write the call in the harness first, and read what it gives.

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
that is a field of an argument, `feed` (the accumulator of `fold`), and the libraries that are not the standard one (a row for
`dio` or `tokio` is the next step; the harness takes them from a lockfile of their own).

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
- Sites (sources and sinks) are listed only in the functions of the project, never inside a dependency (ADR-099; `summary.dependency_sites` says so whenever a body of a dependency was read). A dangerous operation that the library does inside itself is found only when the call into the library has a sink row. So a model is needed for detection as well as for cost.

### When a flow model may stand in for a body

A model that stands in for the body of a dependency (so that the body is not read) is allowed when the body does not exist, when reading it does not converge, when the library calls the user's code (`calls:`), or when reading it costs the analysis a great deal (ADR-092). The last one has two criteria, and a package or a method has to meet one of them.

1. **A whole package** (ADR-092): all of these: (a) the cost is measured and written down with the target it was measured on (a rule of thumb: the dependency is 90% of the time, or gigabytes); (b) a check compares the result with the body read and the result with the model, on a fixture that uses the API: the set of flows and the **edges at the border** (which function of your code a call into the dependency calls back) have to be the same, and the model may only be the side that has more, with a note; (c) every API that calls a function it is given or holds has a `calls:` row (a form that the format cannot write is a problem of the format, and the call site says so, not silence).
2. **One method of an interface with many implementations** (ADR-098): when the implementations delegate to the same method (a clique of delegation, such as `Client.send` of `package:http`), the row of that method is a `flows` model and the bodies of the implementations in dependencies are not read. Two conditions: (a) on public targets the set of findings is the same as with the bodies read, and the targets used are written in the design document; (b) the bodies that were not read are marked with a note of kind `dependency_body_unread`, not silently. The model is only for that method. A thin convenience method that calls it (`get`, `post`) has no model, only a sink if it needs one, and its body is read (cutting it would remove the edge from your code to the call of `send`). The implementations in your own code are read as before. The limit: a function passed to a constructor and called later (`RetryClient(when:)`) cannot be written with `calls:` yet, so only the note appears.

A function of a cut package without a row is read as the unmodelled call it is (its arguments flow to its result, with `unmodeled_library_call`).

### Cutting by package: not available yet

As of today there is no way to cut one package: it is all dependencies or none (`-no-deps`), and a model does not stop the extractor from reading the package. A declaration per package, and not reading the packages that models stand in for from the loading stage on, are **under design** (`docs/DESIGN-package-models.md`, on another branch). They are not decided; do not rely on them. The comparing check of criterion 1(b) does not exist as a tool yet either, and the output does not list the methods that were cut (only the notes on the paths).

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

A new category is made when no other one fits (not for a new library). It is added to this table, and to the decision of the product (`docs/GCP-MODELS.md`, judgement E, for `resource_name`). One symbol has one category (#140).

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
by a function) in it, or be in `bench/parity/dead-allow.txt` with its reason. **A new row is a failing test until a cell
calls it**: add the call to a cell of the zoo, to a function of a curated target, or, for a package of the Dart team, to
`bench/zoo-dart/pure` (`lib/observe.dart` calls the rows of package:path with a marker and `TestZooPathDart` holds the
flows to the run; `lib/probe.dart` only has calls whose spelling is looked at). The IR of real projects is not used for
this: a real project does not use every API, and a row it does not use is not wrong. The file `dead-allow.txt` is for what
a run cannot check (an API that needs a server, a device, a network, or a library the zoo has no dependency on); the lines
marked `debt` are the rows of the standard library that no cell calls yet: they only go down, and none is added for a new
row. The test fails on a line for a row that is hit now, and on one for a row that does not exist.
