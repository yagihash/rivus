# dart-gcp

A synthetic Dart gRPC server that calls the Google Cloud client packages, to measure how `rivus analyze` does on a
project with those dependencies, and whether the paths from a request field to the name of a bucket, an object, a
topic or the text of a query are found. It is a sibling of `../dart-grpc` (which it contains: the three services
`Files`, `Jobs` and `Reports` are the same). It is never run: it is only analyzed. The results are in
`docs/DESIGN-deps-cost.md` (section 10).

The generated code (`lib/src/generated/cloud.*.dart`) was written by hand in the shape of protoc_plugin's output, by a
script that is not kept (`protoc` was not run); `proto/cloud.proto` is for reference. All fields are strings.

| service | packages | methods |
| --- | --- | --- |
| `Gcloud` | `gcloud` (`storage.dart`, `pubsub.dart`, `datastore.dart`) | `ReadObject`, `BucketInfo`, `Publish`, `Query` reach a name or a filter value; `*Fixed` do not |
| `Apis` | `googleapis` (`storage/v1`, `pubsub/v1`, `bigquery/v2`) | `GetObject`, `PublishTopic`, `RunQuery` reach a name or the SQL; `*Fixed` do not |
| `CloudStorage` | `google_cloud_storage` | `Download`, `Upload` reach the bucket and object names; `*Fixed` do not (only the content comes from the request) |
| `Json` | `googleapis`, `google_cloud_protobuf` | the request holds a JSON text that is decoded into a `Map` and given to `fromJson`; the result goes through `toJson` |
| `ManyApis` | `googleapis` (ten more APIs) | one call each, the ids come from the request |

"Reach" means the field of the request is the argument. In a `*Fixed` method the request field is only a key of a
map, or only the content of an upload, and the names are constants.

```sh
cd bench/dart-gcp && dart pub get          # pubspec.lock is committed: the same versions again
RIVUS_DART=dart RIVUS_DART_EXTRACTOR="dart run $PWD/../../extractor/dart/bin/rivus_dart.dart" \
  rivus analyze -lang dart -C . -pass-stats -pass-stats-top 0 -detail summary -timeout 540s
```

The packages are read by the extractor of rivus only. Nothing in them is run (no `dart run`, `dart test` or
`build_runner` on this project). Not in the CI workflows: first the measurement.
