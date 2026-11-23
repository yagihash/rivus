// @cell(F/dart/googleapis-bigquery-table-patch-materialized-view)
// @shape: a value is the SQL that TablesResource.patch of package:googleapis (a skeleton of it) takes in the field `materializedView.query` of its object argument: the sql_injection sink of the built-in rows
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-bigquery-table-patch-materialized-view/vuln', vuln);
  zooCase('F/dart/googleapis-bigquery-table-patch-materialized-view/fixed', fixed);
  zooCase('F/dart/googleapis-bigquery-table-patch-materialized-view/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-table-patch-materialized-view-vuln'); // @source(dart-googleapis-bigquery-table-patch-materialized-view-vuln)
  await api.tables.patch(bq.Table(materializedView: bq.MaterializedViewDefinition(query: 'SELECT * FROM t WHERE a = $v')), 'p', 'd', 't'); // @sink(dart-googleapis-bigquery-table-patch-materialized-view-vuln)
}

Future<void> fixed() async { // @fixed
  final api = bq.BigqueryApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-googleapis-bigquery-table-patch-materialized-view-fixed')))));
  await api.tables.patch(bq.Table(materializedView: bq.MaterializedViewDefinition(query: 'SELECT * FROM t WHERE a = $v')), 'p', 'd', 't'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = bq.BigqueryApi(Object());
  final v = src('dart-googleapis-bigquery-table-patch-materialized-view-decoy'); // @source(dart-googleapis-bigquery-table-patch-materialized-view-decoy)
  await api.tables.patch(bq.Table(materializedView: bq.MaterializedViewDefinition(query: 'SELECT 1'), friendlyName: v), 'p', 'd', 't'); // @sink(none) @nosink(dart-googleapis-bigquery-table-patch-materialized-view-decoy)
  sinkAny(v); // @sink(dart-googleapis-bigquery-table-patch-materialized-view-decoy)
}
