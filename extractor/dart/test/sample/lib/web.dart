import 'dart:io';

// The models of dart:io: a request is attacker controlled, and the dangerous
// calls are found without a rule of the sample.

Future<void> command(HttpRequest req) async { // @source(web-command)
  final cmd = req.uri.queryParameters['cmd']!;
  await Process.run('sh', ['-c', cmd]); // @sink(web-command)
}

void path(HttpRequest req) { // @source(web-path)
  final name = req.uri.path;
  File('/data/' + name).readAsStringSync(); // @sink(web-path)
}

// A percent-encoded value is not hidden from any sink (Uri.encodeComponent neutralises no category: where it lands decides, ADR-114), so the flow is reported with the label url_encoded.
void encoded(HttpRequest req) { // @source(web-encoded)
  final q = Uri.encodeComponent(req.uri.path);
  HttpClient().getUrl(Uri.parse('http://example.com/' + q)); // @sink(web-encoded)
}

void literal() {
  Process.run('ls', ['-l']); // @sink(none)
}

void viaHeader(HttpRequest req) { // @source(web-header)
  final agent = req.headers.value('user-agent')!;
  HttpClient().getUrl(Uri.parse(agent)); // @sink(web-header)
}
