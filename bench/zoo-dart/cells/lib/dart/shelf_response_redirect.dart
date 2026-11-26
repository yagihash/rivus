// @cell(F/dart/shelf-response-redirect)
// @shape: a value from the Request goes into the location of Response.found of package:shelf (a skeleton of it): the open_redirect sink of the built-in rows; the location is read back from the headers to record it (the harness runs the real shelf)
import 'package:shelf/shelf.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/shelf-response-redirect/vuln', vuln);
  zooCase('F/dart/shelf-response-redirect/fixed', fixed);
  zooCase('F/dart/shelf-response-redirect/decoy', decoy);
}

Request _request(String tag) => Request('GET', Uri.parse('http://localhost/?next=' + Uri.encodeQueryComponent(typed(tag))));

Future<void> vuln() async { // @vuln
  Future<Response> handler(Request request) async { // @source(dart-shelf-response-redirect-vuln)
    final next = request.requestedUri.queryParameters['next']!;
    recordAny(Response.found('/go/' + next).headers['location']); // @sink(dart-shelf-response-redirect-vuln)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-redirect-vuln'));
}

Future<void> fixed() async { // @fixed
  Future<Response> handler(Request request) async {
    final next = request.requestedUri.queryParameters['next']!;
    recordAny(Response.found('/go/' + cleanUrl(cleanPath(cleanHtml(cleanSql(cleanCmd(next)))))).headers['location']); // @sink(none)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-redirect-fixed'));
}

Future<void> decoy() async { // @decoy
  Future<Response> handler(Request request) async { // @source(dart-shelf-response-redirect-decoy)
    final next = request.requestedUri.queryParameters['next']!;
    recordAny(Response.found('/home').headers['location']); // @sink(none) @nosink(dart-shelf-response-redirect-decoy)
    sinkSql(next); // @sink(dart-shelf-response-redirect-decoy)
    return Response.ok('');
  }
  await handler(_request('dart-shelf-response-redirect-decoy'));
}
