import 'dart:io';

import 'package:test/test.dart';

// The options of bin/rivus_dart.dart: an option that takes a value and has none, and an option that is not known, stop with the
// status 2 and a usage line; they are not read as the directory to analyse (`--dep-dir` as the last argument was one).
void main() {
  Future<ProcessResult> run(List<String> args) => Process.run(Platform.resolvedExecutable, ['run', 'bin/rivus_dart.dart', ...args]);

  for (final a in [
    ['--dep-dir'],
    ['--allow-deps-under'],
    ['--body-replaced'],
    ['--no-such-option'],
  ]) {
    test('${a.join(' ')} is a usage error', () async {
      final r = await run(a);
      expect(r.exitCode, 2, reason: 'stdout: ${r.stdout}\nstderr: ${r.stderr}');
      expect(r.stderr as String, contains('usage: rivus_dart'));
      expect(r.stdout as String, isEmpty);
    });
  }
}
