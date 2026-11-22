import 'dart:io';

import 'package:grpc/grpc.dart';

import '../generated/files.pbgrpc.dart';

class FilesService extends FilesServiceBase {
  // The only files that Stat answers about.
  static const _known = {
    'config': '/etc/bench/config.json',
    'data': '/var/lib/bench/data.txt',
  };

  @override
  Future<ReadReply> read(ServiceCall call, ReadRequest request) async {
    // vuln: the path of the request is the path of the file (path traversal).
    final text = await File(request.path).readAsString();
    return ReadReply()..data = text;
  }

  @override
  Future<WriteReply> write(
      ServiceCall call, Stream<WriteRequest> request) async {
    var written = 0;
    await for (final r in request) {
      // fixed: the file is a constant; only the content comes from the request.
      await File('/var/lib/bench/data.txt').writeAsString(r.content);
      written += r.content.length;
    }
    return WriteReply()..bytesWritten = written;
  }

  @override
  Future<StatReply> stat(ServiceCall call, StatRequest request) async {
    // fixed: the request picks a key; the path is the value from a table of constants.
    final path = _known[request.path];
    if (path == null) {
      return StatReply()..exists = false;
    }
    final f = File(path);
    return StatReply()
      ..exists = await f.exists()
      ..size = await f.length();
  }
}
