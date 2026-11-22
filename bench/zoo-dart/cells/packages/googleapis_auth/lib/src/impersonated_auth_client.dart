// The skeleton of the impersonation of package:googleapis_auth: it records the account that is impersonated and
// returns an empty client.
import 'package:zoo_mark/zoo_mark.dart';

import 'auth_client.dart';

class ImpersonatedAuthClient implements AuthClient {
  ImpersonatedAuthClient._();
}

Future<ImpersonatedAuthClient> clientViaServiceAccountImpersonation({
  required AuthClient sourceClient,
  required String targetServiceAccount,
  required List<String> targetScopes,
  List<String>? delegates,
}) async {
  recordAny(targetServiceAccount);
  return ImpersonatedAuthClient._();
}
