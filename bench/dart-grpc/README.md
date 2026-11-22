# dart-grpc

A small synthetic Dart gRPC server, to measure how `rivus analyze` does on a project with its dependencies
(`grpc`, `protobuf`, `http2`, and what they import), and whether the paths from a service method to a sink
are found without reading those dependencies (`-no-deps`). The Dart gRPC server is a typical shape of use,
so it is the first target. It is never run: it is only analyzed.

The `*.pb.dart`, `*.pbenum.dart` and `*.pbgrpc.dart` files were **written by hand** in the shape of
protoc_plugin's output (messages extend `GeneratedMessage` with a `BuilderInfo` and `$_getSZ(0)`-style
accessors; services are `FooServiceBase extends Service` with `$addMethod(ServiceMethod(...))`, a `*_Pre`
wrapper and abstract handlers). `protoc` was not run; the `.proto` files in `proto/` are for reference.
The `*.pbjson.dart` files (descriptors) and the clients are left out. The versions of the packages are
those of `../dart-corpus/pubspec.lock`.

```sh
cd bench/dart-grpc && dart pub get --offline
RIVUS_DART_EXTRACTOR="dart run $PWD/../../extractor/dart/bin/rivus_dart.dart" \
  rivus analyze -lang dart -C . -pass-stats            # with the dependencies
RIVUS_DART_EXTRACTOR="..." rivus analyze -lang dart -C . -no-deps -detail snippet
```

| service.method | shape | verdict | what the request reaches |
| --- | --- | --- | --- |
| `Files.Read` | unary | vuln | `File(request.path)` (path traversal) |
| `Files.Write` | client streaming | fixed | only `request.content` is written, to a constant path |
| `Files.Stat` | unary | fixed | `request.path` is a key into a table; the file is the constant from the table |
| `Jobs.Run` | server streaming | vuln | `Process.run(request.command, [request.arguments])` (command injection) |
| `Jobs.Cancel` | unary | fixed | `request.jobId` is a key; `Process.run('kill', ...)` gets the process id from the table |
| `Reports.Fetch` | unary | vuln | `HttpClient().getUrl(Uri.parse(request.url))` (SSRF) |
| `Reports.Lookup` | unary | fixed | `request.key` is only used for a map lookup |

Not in the CI workflows: first the measurement (`docs/DESIGN-deps-cost.md`).
