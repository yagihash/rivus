// A skeleton of package:shelf, made from its public signatures (shelf 1.4). The harness does not run it: exec/pubspec.yaml
// overrides `shelf` with the real package.
import 'dart:async';

import 'request.dart';
import 'response.dart';

typedef Handler = FutureOr<Response> Function(Request request);
