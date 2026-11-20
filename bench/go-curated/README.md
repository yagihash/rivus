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
* `@miss` after a `@source(...)`: the flow is there, and the engine does not find it yet.
  `@fp` after a `@sink(none)` or a `@nosink(...)`: the engine reports a flow that is not there.
  These two record what the engine cannot do yet, so that this is a ratchet: they are checked from
  both sides, and when a change of the engine makes a mark untrue (the flow is found, the false
  report is gone) the test fails and the mark is to be removed. A mark is never a reason to
  loosen a check: a new `@miss`/`@fp` needs a reason in the comment next to it.

`rules.yaml` names the sources (`Context.Query` ...), sinks (`DB.Exec`, `sys.Run`, `Context.Redirect`,
`Context.HTML`) and sanitizers (`valid.Ident`, `valid.Quote`) of the project. The built-in rules are
in force as always.

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
