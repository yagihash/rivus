// @cell(F/dart/shelf-response-body)
// @shape: a value from the Request goes into the body of a Response of package:shelf (a skeleton of it): the xss sink of the built-in rows; the body is read back to record it (the harness runs the real shelf, which records nothing itself)
import 'package:shelf/shelf.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/shelf-response-body/vuln', vuln);
  zooCase('F/dart/shelf-response-body/fixed', fixed);
  zooCase('F/dart/shelf-response-body/decoy', decoy);
}

Request _request(String tag) => Request('GET', Uri.parse('http://localhost/?q=' + Uri.encodeQueryComponent(typed(tag))));

Future<void> vuln() async { // @vuln
  Future<Response> handler(Request request) async { // @source(dart-shelf-response-body-vuln)
    final q = request.requestedUri.queryParameters['q']!;
    recordAny(await Response.ok(q).readAsString()); // @sink(dart-shelf-response-body-vuln)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-body-vuln'));
}

Future<void> fixed() async { // @fixed
  Future<Response> handler(Request request) async {
    final q = request.requestedUri.queryParameters['q']!;
    recordAny(await Response.ok(cleanPath(cleanHtml(cleanSql(cleanCmd(q))))).readAsString()); // @sink(none)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-body-fixed'));
}

Future<void> decoy() async { // @decoy
  Future<Response> handler(Request request) async { // @source(dart-shelf-response-body-decoy)
    final q = request.requestedUri.queryParameters['q']!;
    recordAny(await Response.ok('constant').readAsString()); // @sink(none) @nosink(dart-shelf-response-body-decoy)
    sinkSql(q); // @sink(dart-shelf-response-body-decoy)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-body-decoy'));
}
