// @cell(F/dart/googleapis-bigquery-routine-insert-body)
// @shape: a value is the SQL that RoutinesResource.insert of package:googleapis (a skeleton of it) takes in the field `definitionBody` of its object argument: the sql_injection sink of the built-in rows
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-bigquery-routine-insert-body/vuln', vuln);
  zooCase('F/dart/googleapis-bigquery-routine-insert-body/fixed', fixed);
  zooCase('F/dart/googleapis-bigquery-routine-insert-body/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-routine-insert-body-vuln'); // @source(dart-googleapis-bigquery-routine-insert-body-vuln)
  await api.routines.insert(bq.Routine(definitionBody: 'SELECT * FROM t WHERE a = $v'), 'p', 'd'); // @sink(dart-googleapis-bigquery-routine-insert-body-vuln)
}

Future<void> fixed() async { // @fixed
  final api = bq.BigqueryApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-googleapis-bigquery-routine-insert-body-fixed')))));
  await api.routines.insert(bq.Routine(definitionBody: 'SELECT * FROM t WHERE a = $v'), 'p', 'd'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-routine-insert-body-decoy'); // @source(dart-googleapis-bigquery-routine-insert-body-decoy)
  await api.routines.insert(bq.Routine(definitionBody: 'SELECT 1', description: v), 'p', 'd'); // @sink(none) @nosink(dart-googleapis-bigquery-routine-insert-body-decoy)
  sinkAny(v); // @sink(dart-googleapis-bigquery-routine-insert-body-decoy)
}
