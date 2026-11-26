// @cell(F/dart/googleapis-bigquery-table-update-view)
// Go side: not measured yet (no Go cell of this form in zoo-go-pkg or bench/gcp); not a form that Go lacks. Counted as a Go todo by TestZooDart (no mark)
// @shape: a value is the SQL that TablesResource.update of package:googleapis (a skeleton of it) takes in the field `view.query` of its object argument: the sql_injection sink of the built-in rows
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-bigquery-table-update-view/vuln', vuln);
  zooCase('F/dart/googleapis-bigquery-table-update-view/fixed', fixed);
  zooCase('F/dart/googleapis-bigquery-table-update-view/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-table-update-view-vuln'); // @source(dart-googleapis-bigquery-table-update-view-vuln)
  await api.tables.update(bq.Table(view: bq.ViewDefinition(query: 'SELECT * FROM t WHERE a = $v')), 'p', 'd', 't'); // @sink(dart-googleapis-bigquery-table-update-view-vuln)
}

Future<void> fixed() async { // @fixed
  final api = bq.BigqueryApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-bigquery-table-update-view-fixed')))));
  await api.tables.update(bq.Table(view: bq.ViewDefinition(query: 'SELECT * FROM t WHERE a = $v')), 'p', 'd', 't'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-table-update-view-decoy'); // @source(dart-googleapis-bigquery-table-update-view-decoy)
  await api.tables.update(bq.Table(view: bq.ViewDefinition(query: 'SELECT 1'), description: v), 'p', 'd', 't'); // @sink(none) @nosink(dart-googleapis-bigquery-table-update-view-decoy)
  sinkAny(v); // @sink(dart-googleapis-bigquery-table-update-view-decoy)
}
