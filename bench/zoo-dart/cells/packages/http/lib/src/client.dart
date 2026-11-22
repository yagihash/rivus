// The skeleton of `Client` of package:http: an interface whose methods take the URL first. It is real code, not a
// declaration: the harness runs it and it records the URL that a request would go to (the text of the URL as it is
// sent, with the percent-encoding undone) as what reached the sink; rivus does not read it (the built-in rows of
// package:http stand for the body of a dependency's method).
import 'package:zoo_mark/zoo_mark.dart';

import 'response.dart';

abstract interface class Client {
  factory Client() => _SkeletonClient();

  Future<Response> get(Uri url, {Map<String, String>? headers});
}

class _SkeletonClient implements Client {
  @override
  Future<Response> get(Uri url, {Map<String, String>? headers}) async {
    recordAny(Uri.decodeFull(url.toString()));
    return Response('', 200);
  }
}
