// @cell(F/dart/googleapis-datastore-run-query)
// @shape: a value is the text of the GqlQuery that ProjectsResource.runQuery of package:googleapis (a skeleton of it) runs: the sql_injection sink of the built-in rows (the field gqlQuery.queryString); the bindings of a GqlQuery are values that the service binds
import 'package:googleapis/datastore/v1.dart' as ds;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-datastore-run-query/vuln', vuln);
  zooCase('F/dart/googleapis-datastore-run-query/fixed', fixed);
  zooCase('F/dart/googleapis-datastore-run-query/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = ds.DatastoreApi(Object());
  final v = src('dart-googleapis-datastore-run-query-vuln'); // @source(dart-googleapis-datastore-run-query-vuln)
  await api.projects.runQuery(ds.RunQueryRequest(gqlQuery: ds.GqlQuery(queryString: 'SELECT * FROM T WHERE a = $v')), 'p'); // @sink(dart-googleapis-datastore-run-query-vuln)
}

Future<void> fixed() async { // @fixed
  final api = ds.DatastoreApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-datastore-run-query-fixed')))));
  await api.projects.runQuery(ds.RunQueryRequest(gqlQuery: ds.GqlQuery(queryString: 'SELECT * FROM T WHERE a = $v')), 'p'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = ds.DatastoreApi(Object());
  final v = src('dart-googleapis-datastore-run-query-decoy'); // @source(dart-googleapis-datastore-run-query-decoy)
  await api.projects.runQuery(ds.RunQueryRequest(gqlQuery: ds.GqlQuery(queryString: 'SELECT * FROM T WHERE a = @a', namedBindings: {'a': ds.GqlQueryParameter(value: v)})), 'p'); // @sink(none) @fp @nosink(dart-googleapis-datastore-run-query-decoy) @fp @ref(#141)
  sinkAny(v); // @sink(dart-googleapis-datastore-run-query-decoy)
}
