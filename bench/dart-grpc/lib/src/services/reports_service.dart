import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';

import '../generated/reports.pbgrpc.dart';

class ReportsService extends ReportsServiceBase {
  final Map<String, String> _cache = {};

  @override
  Future<FetchReply> fetch(ServiceCall call, FetchRequest request) async {
    // vuln: the URL of the request is the address of the outgoing request (SSRF).
    final client = HttpClient();
    final req = await client.getUrl(Uri.parse(request.url));
    final res = await req.close();
    final body = await res.transform(utf8.decoder).join();
    client.close();
    return FetchReply()
      ..status = res.statusCode
      ..body = body;
  }

  @override
  Future<LookupReply> lookup(ServiceCall call, LookupRequest request) async {
    // fixed: the key is only used to look up a value; it reaches no sink.
    final value = _cache[request.key] ?? '';
    return LookupReply()..value = value;
  }
}
