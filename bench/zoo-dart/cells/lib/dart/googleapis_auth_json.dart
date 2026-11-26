// @cell(F/dart/googleapis-auth-json)
// Go side: not measured yet (no Go cell of this form in zoo-go-pkg or bench/gcp); not a form that Go lacks. Counted as a Go todo by TestZooDart (no mark)
// @shape: a value goes into the JSON of ServiceAccountCredentials.fromJson of package:googleapis_auth (a skeleton of it): the resource_name sink of the built-in rows
import 'package:googleapis_auth/auth_io.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-auth-json/vuln', vuln);
  zooCase('F/dart/googleapis-auth-json/fixed', fixed);
  zooCase('F/dart/googleapis-auth-json/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final v = src('dart-googleapis-auth-json-vuln'); // @source(dart-googleapis-auth-json-vuln)
  await Future.value(ServiceAccountCredentials.fromJson(v)); // @sink(dart-googleapis-auth-json-vuln)
}

Future<void> fixed() async { // @fixed
  final v = cleanResource(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-auth-json-fixed'))))));
  await Future.value(ServiceAccountCredentials.fromJson(v)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final v = src('dart-googleapis-auth-json-decoy'); // @source(dart-googleapis-auth-json-decoy)
  await Future.value(ServiceAccountCredentials.fromJson('{}')); // @sink(none) @nosink(dart-googleapis-auth-json-decoy)
  sinkAny(v); // @sink(dart-googleapis-auth-json-decoy)
}
