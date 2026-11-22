// The skeleton of `ServiceAccountCredentials` of package:googleapis_auth: real code (the harness runs it), not read by
// rivus (the built-in rows stand for it). It records the e-mail, and keeps the JSON as it is (the real one parses it).
import 'package:zoo_mark/zoo_mark.dart';

import 'client_id.dart';

class ServiceAccountCredentials {
  ServiceAccountCredentials(String email, ClientId clientId, String privateKey, {String? impersonatedUser}) : email = email {
    recordAny(email);
  }

  factory ServiceAccountCredentials.fromJson(Object? json, {String? impersonatedUser}) {
    recordAny(json);
    return ServiceAccountCredentials('', ClientId(''), '');
  }

  final String email;
}
