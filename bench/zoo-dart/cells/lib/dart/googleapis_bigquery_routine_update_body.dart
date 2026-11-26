// @cell(F/dart/googleapis-bigquery-routine-update-body)
// Go side: not measured yet (no Go cell of this form in zoo-go-pkg or bench/gcp); not a form that Go lacks. Counted as a Go todo by TestZooDart (no mark)
// @shape: a value is the SQL that RoutinesResource.update of package:googleapis (a skeleton of it) takes in the field `definitionBody` of its object argument: the sql_injection sink of the built-in rows
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-bigquery-routine-update-body/vuln', vuln);
  zooCase('F/dart/googleapis-bigquery-routine-update-body/fixed', fixed);
  zooCase('F/dart/googleapis-bigquery-routine-update-body/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-routine-update-body-vuln'); // @source(dart-googleapis-bigquery-routine-update-body-vuln)
  await api.routines.update(bq.Routine(definitionBody: 'SELECT * FROM t WHERE a = $v'), 'p', 'd', 'r'); // @sink(dart-googleapis-bigquery-routine-update-body-vuln)
}

Future<void> fixed() async { // @fixed
  final api = bq.BigqueryApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-bigquery-routine-update-body-fixed')))));
  await api.routines.update(bq.Routine(definitionBody: 'SELECT * FROM t WHERE a = $v'), 'p', 'd', 'r'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-routine-update-body-decoy'); // @source(dart-googleapis-bigquery-routine-update-body-decoy)
  await api.routines.update(bq.Routine(definitionBody: 'SELECT 1', description: v), 'p', 'd', 'r'); // @sink(none) @nosink(dart-googleapis-bigquery-routine-update-body-decoy)
  sinkAny(v); // @sink(dart-googleapis-bigquery-routine-update-body-decoy)
}
