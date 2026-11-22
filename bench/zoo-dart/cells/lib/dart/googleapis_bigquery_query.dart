// @cell(F/dart/googleapis-bigquery-query)
// @shape: a value is the text of the QueryRequest that JobsResource.query of package:googleapis (a skeleton of it) runs: the sql_injection sink of the built-in rows (the field `query` of the request)
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-bigquery-query/vuln', vuln);
  zooCase('F/dart/googleapis-bigquery-query/fixed', fixed);
  zooCase('F/dart/googleapis-bigquery-query/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-query-vuln'); // @source(dart-googleapis-bigquery-query-vuln)
  await api.jobs.query(bq.QueryRequest(query: 'SELECT * FROM t WHERE a = $v'), 'p'); // @sink(dart-googleapis-bigquery-query-vuln)
}

Future<void> fixed() async { // @fixed
  final api = bq.BigqueryApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-bigquery-query-fixed')))));
  await api.jobs.query(bq.QueryRequest(query: 'SELECT * FROM t WHERE a = $v'), 'p'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-query-decoy'); // @source(dart-googleapis-bigquery-query-decoy)
  await api.jobs.query(bq.QueryRequest(query: 'SELECT 1', location: v), 'p'); // @sink(none) @nosink(dart-googleapis-bigquery-query-decoy)
  sinkAny(v); // @sink(dart-googleapis-bigquery-query-decoy)
}
