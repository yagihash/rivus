// @cell(F/dart/http-ioclient-send)
// @shape: a value goes into the request that is sent by a variable typed IOClient (a class of package:http, a skeleton of it), not by the interface Client
import 'package:http/http.dart' as http;
import 'package:http/io_client.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/http-ioclient-send/vuln', vuln);
  zooCase('F/dart/http-ioclient-send/fixed', fixed);
  zooCase('F/dart/http-ioclient-send/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final client = IOClient();
  final v = src('dart-http-ioclient-send-vuln'); // @source(dart-http-ioclient-send-vuln)
  await client.send(http.Request('GET', Uri.parse('http://example.invalid/?q=$v'))); // @sink(dart-http-ioclient-send-vuln)
}

Future<void> fixed() async { // @fixed
  final client = IOClient();
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-http-ioclient-send-fixed')))));
  await client.send(http.Request('GET', Uri.parse('http://example.invalid/?q=$v'))); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final client = IOClient();
  final v = src('dart-http-ioclient-send-decoy'); // @source(dart-http-ioclient-send-decoy)
  await client.send(http.Request('GET', Uri.parse('http://example.invalid/'))); // @sink(none) @nosink(dart-http-ioclient-send-decoy)
  sinkAny(v); // @sink(dart-http-ioclient-send-decoy)
}
