import 'dart:io';

import 'package:grpc/grpc.dart';

import '../generated/jobs.pbgrpc.dart';

class JobsService extends JobsServiceBase {
  final Map<String, int> _pids = {};

  @override
  Stream<RunReply> run(ServiceCall call, RunRequest request) async* {
    // vuln: the command and its argument come from the request (command injection).
    final result = await Process.run(request.command, [request.arguments]);
    _pids[request.command] = result.pid;
    for (final line in (result.stdout as String).split('\n')) {
      yield RunReply()
        ..line = line
        ..state = JobState.RUNNING;
    }
    yield RunReply()
      ..exitCode = result.exitCode
      ..state = result.exitCode == 0 ? JobState.DONE : JobState.FAILED;
  }

  @override
  Future<CancelReply> cancel(ServiceCall call, CancelRequest request) async {
    // fixed: the request picks a job; the process id is another value, from the table.
    final pid = _pids[request.jobId];
    if (pid == null) {
      return CancelReply()..ok = false;
    }
    await Process.run('kill', ['-TERM', '$pid']);
    return CancelReply()..ok = true;
  }
}
