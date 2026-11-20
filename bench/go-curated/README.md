# go-curated: accuracy regression for Go (with answers)

A small Go project (about 20 files, one module, no dependencies) that has the habits of the real
projects the engine is used on: the Gin/Macaron style of handlers (a context, router groups, middleware
chains), an ORM with `Where`/`Exec`, a service layer behind interfaces with two implementations,
dependency injection, closures, function values in maps and struct fields, functional options, model
types behind an alias (`type User = models.User`), embedding, generics, defer, channels, a recursive
tree, errors that wrap, stored data (not a source), settings from a package variable.

The answers are in the source, in the way of `testdata/go/cases` and of the Dart/Rust samples:

```go
name := c.Query("name")                                       // @source(direct)
h.DB.Exec("SELECT * FROM users WHERE name = '" + name + "'")  // @sink(direct)
h.DB.Exec("SELECT * FROM " + config.Default.Table)            // @sink(none)
```

* `@source(tag)` and `@sink(tag)`: the data of the source reaches the sink. A tag has one source and
  one or more sinks.
* `@sink(none)`: nothing reaches this sink (a sanitizer, a constant, a number, stored data).
* `@nosink(tag)`: the source of the tag must not reach this line, which has a flow of its own from
  another source (two keys of one map, say).
* `@miss(silent)` or `@miss(<kind>)` after a `@source(...)`: the flow is there, and the engine does not
  find it yet. `silent` says that the engine left no uncertainty in the function of the source or the
  function of the sink (the test fails when one comes: write its kind, it is a gain); a kind
  (`unresolved_callee`, `unmodeled_library_call`, `dynamic_dispatch`, `depth_limit`; several with a comma)
  says which uncertainty is there (the test fails when it is gone). A `dynamic_dispatch` with exactly one
  candidate is not a trace (see [`../MISSES.md`](../MISSES.md)).
  `@fp` after a `@sink(none)` or a `@nosink(...)`: the engine reports a flow that is not there.
  These two record what the engine cannot do yet, so that this is a ratchet: they are checked from
  both sides, and when a change of the engine makes a mark untrue (the flow is found, the false
  report is gone) the test fails and the mark is to be removed. A mark is never a reason to
  loosen a check: a new `@miss`/`@fp` needs a reason in the comment next to it.

`rules.yaml` names the sources (`Context.Query` ...), sinks (`DB.Exec`, `sys.Run`, `Context.Redirect`,
`Context.HTML`) and sanitizers (`valid.Ident`, `valid.Quote`) of the project. The built-in rules are
in force as always.

## The shapes of the CVEs of the real benchmarks

`gitea/`, `gva/`, `gogs/` and `ext/` (a dependency whose entry point is the sink) hold, in a small form, the
flow from the source to the sink of the ground truths in `bench/gitea/truth-v1.26.4.json`,
`bench/gin-vue-admin/truth.json` and `bench/gogs/truth-*.json`: the structure (the handler, the context, the
helpers that it goes through, the interface with two implementations, the `append` in a loop, the
dependency) and not the code. They are the way to notice, in CI, that a flow of Gitea is lost; the real
project is not in CI. The rules for them are at the end of `rules.yaml`.

| tag | what | found |
| --- | --- | --- |
| `release_del` | GHSA-4p89: the keys of the form (`attachment-del-<uuid>`) are collected with `append` in a loop, go to a service with six parameters, and the files are deleted by the UUIDs after a `db.WithTx` closure; `storage.Attachments.Delete` is an interface with two implementations | yes (two sinks: the call and `os.Remove` of the local storage) |
| `oauth_avatar` | GHSA-x77v: `goth.User` (a struct by value) to `http.NewRequestWithContext` | yes |
| `openid_verify` | GHSA-8vmh: the URL of the request, through the wrapper, to the library's `Verify` (the entry point is the sink) | yes |
| `gva_plug` | GHSA-gv3w: `c.Bind(&req)`, a field made of another, the struct by value, `filepath.Join` + `fmt.Sprintf`, `os.MkdirAll` and `os.OpenFile` | yes |
| `gogs_put`, `gogs_hook` | GHSA-qf5v (`c.Param("*")` into an options struct, `path.Join`, `os.WriteFile`) and the hook name into `Repository.Hook` of a dependency | yes |
| `tx_capture`, `tx_capture_plain` | the way every Gitea service is written: the work is a closure given to `db.WithTx`, and it uses a captured value | **no** (`@miss(dynamic_dispatch)` and `@miss(silent)`): see [`../MISSES.md`](../MISSES.md) |

## The values a closure captures (`api/capture.go`)

A closure that captures a value and is called by a helper is the form that every Gitea service has
(`tx_capture`, above). `api/capture.go` has the forms around it, so that a change of the engine (C-7:
the captured values are applied where the closure is made, `docs/DESIGN-capture.md`) is checked from
both sides: the helper with two callers, each with its own closure (`cap-run-a`, `cap-run-b`: the value
of one does not reach the closure of the other, `@nosink`); a write to a captured variable (`cap-write`);
a closure in a closure (`cap-nested`); a closure made in a loop (`cap-loop`); two closures of one call
(`cap-two-a`); a closure that a function returns (`cap-returned`). Seven are `@miss(silent)` now. The
forms that are found now are kept as the control: the closure that gets its value by its argument, the
closure called where it is made, `go`, `defer`. The closure that uses its own parameter, not what it
captured (`cap-param`), and the closure that writes a constant (`cap-write-const`) are `@sink(none)`.
`cap-never` is a closure that is made and not called: quiet now, and the one that C-7 would report (a
limit of that rule: the line is to become `@fp`, or the rule is changed).

## What is not found yet (`@miss`) and what is reported wrongly (`@fp`)

The three `@miss` (`options`: a closure returned by a function, in a variadic list, loses what it captured, `@miss(silent)`; `tx_capture` and `tx_capture_plain`: a closure that captures a value and is called by a helper (`db.WithTx`), one with a trace, `@miss(dynamic_dispatch)`, one `@miss(silent)`) and the five `@fp` (a map that does not tell its keys apart: four; a switch allow-list: one) are classified by the cause in [`../MISSES.md`](../MISSES.md).

The findings whose path has no step in the function of their source are counted too (`offSourcePath`,
BENCH-2: 0 here; a ceiling, see [`../MISSES.md`](../MISSES.md)).

## Run it

```sh
go test ./cmd/rivus -run TestCuratedGo -v       # a second or two; it is part of "go test ./..."
```

The test (`cmd/rivus/bench_curated_test.go`) builds the binary, runs `rivus analyze -rules rules.yaml
./...` in this directory and compares the findings with the annotations. `checkCurated` takes a
directory, so that the Dart and Rust projects (`bench/dart-curated`, `bench/rust-curated`) can use the
same checker with their own rules and comment syntax.

This directory is a module of its own (`go.mod`), so that `go build ./...` and `go test ./...` of the
repository do not compile it as part of rivus.

## Adding a case

Write the handler the way the real project would, in the file of its kind; register the route in
`routers/routers.go` so that it is reached from the program; tag it. Mark the line of what you want
to be quiet `@sink(none)`. If the engine gets it wrong, say so with `@miss` or `@fp`, and say in the
issue tracker why. Keep the count of the cases that are not marked up (the test checks a floor).
