// @cell(F/dart/gcloud-datastore-query-kind)
// @shape: a value is the kind of a Query (a named argument of its constructor) that Datastore.query of package:gcloud (a skeleton of it) runs: a name that a row of the built-in model cannot point to
import 'package:gcloud/datastore.dart' as ds;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/gcloud-datastore-query-kind/vuln', vuln);
  zooCase('F/dart/gcloud-datastore-query-kind/fixed', fixed);
  zooCase('F/dart/gcloud-datastore-query-kind/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final store = ds.Datastore(Object(), 'p');
  final v = src('dart-gcloud-datastore-query-kind-vuln'); // @source(dart-gcloud-datastore-query-kind-vuln) @miss(unmodeled_library_call) @ref(#139)
  await store.query(ds.Query(kind: v)); // @sink(dart-gcloud-datastore-query-kind-vuln)
}

Future<void> fixed() async { // @fixed
  final store = ds.Datastore(Object(), 'p');
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-gcloud-datastore-query-kind-fixed')))));
  await store.query(ds.Query(kind: v)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final store = ds.Datastore(Object(), 'p');
  final v = src('dart-gcloud-datastore-query-kind-decoy'); // @source(dart-gcloud-datastore-query-kind-decoy)
  await store.query(ds.Query(kind: 'Item', filters: [ds.Filter('=', 'name', v)])); // @sink(none) @nosink(dart-gcloud-datastore-query-kind-decoy)
  sinkAny(v); // @sink(dart-gcloud-datastore-query-kind-decoy)
}
