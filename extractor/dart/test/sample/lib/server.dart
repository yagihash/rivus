import 'package:http/http.dart' as http;
import 'package:shelf/shelf.dart';

// The models of package:shelf and package:http.

Future<Response> proxy(Request request) async { // @source(shelf-ssrf)
  final target = request.url.queryParameters['url']!;
  final res = await http.get(Uri.parse(target)); // @sink(shelf-ssrf)
  return Response.ok(res.body);
}

Future<Response> echo(Request request) async { // @source(shelf-body)
  final text = await request.readAsString();
  await http.post(Uri.parse('http://example.com/' + text)); // @sink(shelf-body)
  return Response.ok('');
}

Future<Response> fixed(Request request) async {
  await http.get(Uri.parse('http://example.com/health')); // @sink(none)
  return Response.ok('ok');
}
