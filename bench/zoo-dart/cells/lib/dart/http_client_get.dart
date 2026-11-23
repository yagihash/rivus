// @cell(F/dart/http-client-get)
// @shape: a value goes into the URL of Client.get of package:http (a skeleton of it): the SSRF sink of the built-in rows
import 'package:http/http.dart' as http;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/http-client-get/vuln', vuln);
  zooCase('F/dart/http-client-get/fixed', fixed);
  zooCase('F/dart/http-client-get/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final client = http.Client();
  final v = src('dart-http-client-get-vuln'); // @source(dart-http-client-get-vuln)
  await client.get(Uri.parse('http://example.invalid/?q=$v')); // @sink(dart-http-client-get-vuln)
}

Future<void> fixed() async { // @fixed
  final client = http.Client();
  final v = cleanUrl(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-http-client-get-fixed'))))));
  await client.get(Uri.parse('http://example.invalid/?q=$v')); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final client = http.Client();
  final v = src('dart-http-client-get-decoy'); // @source(dart-http-client-get-decoy)
  await client.get(Uri.parse('http://example.invalid/'), headers: {'x': v}); // @sink(none) @nosink(dart-http-client-get-decoy)
  sinkAny(v); // @sink(dart-http-client-get-decoy)
}
