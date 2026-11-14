import 'user.dart';

export 'user.dart';

/// Reads the query of a request: the source of taint in this example.
String readQuery(Map<String, String> request) => request['q'] ?? '';

/// Builds a user from JSON through generated code.
User parseUser(Map<String, dynamic> json) => User.fromJson(json);
