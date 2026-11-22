// @cell(F/dart/googleapis-auth-email)
// @shape: a value goes into the e-mail of a ServiceAccountCredentials of package:googleapis_auth (a skeleton of it): the resource_name sink of the built-in rows
import 'package:googleapis_auth/auth_io.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-auth-email/vuln', vuln);
  zooCase('F/dart/googleapis-auth-email/fixed', fixed);
  zooCase('F/dart/googleapis-auth-email/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final v = src('dart-googleapis-auth-email-vuln'); // @source(dart-googleapis-auth-email-vuln)
  await Future.value(ServiceAccountCredentials(v, ClientId('i'), 'k')); // @sink(dart-googleapis-auth-email-vuln)
}

Future<void> fixed() async { // @fixed
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-auth-email-fixed')))));
  await Future.value(ServiceAccountCredentials(v, ClientId('i'), 'k')); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final v = src('dart-googleapis-auth-email-decoy'); // @source(dart-googleapis-auth-email-decoy)
  await Future.value(ServiceAccountCredentials('target-account', ClientId('i'), 'k')); // @sink(none) @nosink(dart-googleapis-auth-email-decoy)
  sinkAny(v); // @sink(dart-googleapis-auth-email-decoy)
}
