// @cell(F/dart/googleapis-auth-impersonate)
// @shape: a value goes into targetServiceAccount of clientViaServiceAccountImpersonation of package:googleapis_auth (a skeleton of it): the resource_name sink of the built-in rows
import 'package:googleapis_auth/auth_io.dart';
import 'package:zoo_mark/zoo_mark.dart';

final AuthClient base = BaseClient();

class BaseClient implements AuthClient {}

void register() {
  zooCase('F/dart/googleapis-auth-impersonate/vuln', vuln);
  zooCase('F/dart/googleapis-auth-impersonate/fixed', fixed);
  zooCase('F/dart/googleapis-auth-impersonate/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final v = src('dart-googleapis-auth-impersonate-vuln'); // @source(dart-googleapis-auth-impersonate-vuln)
  await clientViaServiceAccountImpersonation(sourceClient: base, targetServiceAccount: v, targetScopes: ['s']); // @sink(dart-googleapis-auth-impersonate-vuln)
}

Future<void> fixed() async { // @fixed
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-googleapis-auth-impersonate-fixed')))));
  await clientViaServiceAccountImpersonation(sourceClient: base, targetServiceAccount: v, targetScopes: ['s']); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final v = src('dart-googleapis-auth-impersonate-decoy'); // @source(dart-googleapis-auth-impersonate-decoy)
  await clientViaServiceAccountImpersonation(sourceClient: base, targetServiceAccount: 'target-account', targetScopes: ['s']); // @sink(none) @nosink(dart-googleapis-auth-impersonate-decoy)
  sinkAny(v); // @sink(dart-googleapis-auth-impersonate-decoy)
}
