# The Dart extractor of rivus

Reads Dart code with `package:analyzer` and writes the neutral IR
(`docs/ARCHITECTURE.md` 2, the stream of 2.6 and the texts of 2.7.7) as JSON Lines.
The core reads it with `rivus analyze -ir` and `rivus sites -ir`.

```sh
cd extractor/dart
dart pub get
dart run bin/rivus_dart.dart path/to/package > package.jsonl
rivus analyze -ir package.jsonl -rules rules.yaml -no-builtin-rules
```

The libraries the code imports (`package:` dependencies and what they import, not the SDK) are
read too, and their functions are marked `dep`: `rivus analyze` keeps the ones that the code
reaches, so that a flow is followed through a dependency instead of guessed at. `--no-deps`
leaves them out.

The files that a package's build does not ship are not read: `test/`, `integration_test/`, `benchmark/`,
`tool/` and `example/` of each package (a directory with a `pubspec.yaml`; a `lib/src/test/` is code of the
library). The dependencies are followed from the code that is read, so a package declared under
`dev_dependencies` that no shipped file imports is not read either. The stream says how many files and
packages were left out (`tests_skipped`). `--include-tests` (like `rivus analyze -tests`) reads them all.
A shipped file that imports one of them reads it, as a dependency (ADR-062).

The package needs its `dart pub get` done (and `build_runner` if it has generated
code): that is what makes `package:` imports and `part` files resolve.

`-C` may point at one package, or at the parent of several (no `pubspec.yaml` above them). In the second
case a dependency is resolved by the `package_config.json` of the package that imports it, so its functions
are named by `package:` URIs as with one package. A dependency that is still not (a `file:` URI), or that two
packages of the tree read in different versions (the functions would have the same IDs), is said in
`summary.unread`; point `-C` at one package, or run `dart pub get` in each package.

Test: `RIVUS_DART="dart" go test -run TestDartExtractor ./cmd/rivus` (or
`RIVUS_DART="mise exec dart@3.13.6 -- dart"`). The sample is `test/sample`: every
flow the annotations (`@source(tag)`, `@sink(tag)`) declare is found, and no sink
marked `@sink(none)` is reached.

## Symbols

The rules name Dart things as `<library uri>#<Type>.<member>`:

| thing | symbol |
|---|---|
| top level function, variable | `package:app/a.dart#parse` |
| method, getter, setter, field | `package:app/a.dart#Request.url` |
| constructor | `package:app/a.dart#Request.new`, `...#Request.fromJson` |
| closure | `<enclosing function>$1` |

Field rules use the same form (`field: package:app/a.dart#Request.url`).

## How Dart is lowered (what is decided)

- Variables are mutable locals. A variable that a closure captures **and** that is
  assigned after its declaration lives in a cell (`alloc`, `load`, `store`); the cell
  is what the closure captures.
- A call that may throw, inside a `try`, ends its block, with an edge for the
  exception and one for the continuation. `finally` is copied once for every way out.
- `await` ends the block with an ordinary edge, and sets the `await` flag of the call.
- `&&`, `||`, `?:`, `??` are control flow, joined in a local.
- String interpolation is a chain of `+` of the constant parts and the expressions.
- A named parameter is placed after the positional ones, **by name** (so that a method
  and its overrides line up). A call leaves out what it does not pass.
- A class instance is a `pointer`; `List`, `Set`, `Map`, `Iterable` are `container`.
- Calls on an instance go through the receiver (`dynamic`), with the candidates found
  by class hierarchy analysis over the code that was read.
- Patterns (`switch`, `if-case`, destructuring, switch expressions) are tests on the
  value, reads of its parts (`field_load`, `index_load`) and the variables they bind.
  Records are values with fields `$1`, `$2`, ... and by name.
- A constructor is a function that allocates the object, sets the fields
  (initializers of the class, `this.x` parameters, the initializer list) and returns it.
- `for (x in xs)` is `iter.start` / `iter.next` (the symbols of the engine).

## Models

The built-in rule sets `dart-*.yaml` (`internal/model/builtin`) cover `dart:core`,
`dart:convert`, `dart:io`, `dart:html` and, for `package:shelf`, `package:http` and
`sqflite`, the sources and the sinks. An SDK library that others export is named by the
one people import (`dart:_http` is `dart:io`). A sink takes `accepts: [user_input,
file_content]`, so that the label a sanitizer adds does not make it fire.

## Not done

- **Field initializers** of an enum are set by the constructors it declares (an enum that declares none gets
  none; its fields are const, so they hold no data of the program). The initializers of a mixin are set by the
  constructors of the class that uses it when the mixin is in the same file; a mixin of another file says
  `unsupported`.
- A pattern's shape tests (the length of a list, whether a key exists) are not made; the
  type test of an object pattern and the constants are.
- **Calls through function values**: the core finds the functions that flow to the value
  through copies, φ functions and the arguments of the callers of the parameter. A
  function kept in a field or a list is not found. A closure found from another function does not get
  the values it captured (they carry nothing); one made in the function that calls it does.
- **Streams**: what a generator (`async*`, `sync*`) yields is collected in its result, so
  `await for` and `for` over a call of it see the values. A stream passed through a
  `StreamController` is modeled (`dart-async.yaml`); `Stream.listen`, `Future.then` and the
  callbacks of `Iterable` (`forEach`, `map`, `where`, ...) are called with what the receiver holds.
- **Named parameters** of an override that adds one the base does not have may shift
  the others (by name, the usual case lines up).
- **Dependencies** are read by default (`--no-deps` leaves them out; `dart:*` is not read): calls into a
  package are followed instead of guessed. Where a dependency is, is said by `.dart_tool/package_config.json`,
  which is part of the project and is not trusted: it can point at any directory. What is under the project, the
  pub cache (`PUB_CACHE`, or `~/.pub-cache`), the SDK that runs the extractor and the Flutter that holds it
  (`FLUTTER_ROOT`) is read; anything else is left out and the stream says so (a `load_error` that names the
  directory). `--allow-deps-under DIR` (repeatable; `rivus analyze -allow-deps-under DIR`) reads the packages
  under `DIR` too: a monorepo whose packages are siblings of the one that is analysed needs it.