import 'dart:convert';
import 'dart:io';

import 'package:rivus_dart/extract.dart';
import 'package:path/path.dart' as p;
import 'package:test/test.dart';

// The Go tests (cmd/rivus) check what the extractor finds; this one only checks that it runs on the sample
// and writes JSON Lines. The sample must have had `dart pub get` (see the README).
void main() {
  test('extract writes JSON Lines for the sample', () async {
    final out = StringBuffer();
    await extract('test/sample', out, deps: false);
    final lines = const LineSplitter().convert(out.toString());
    expect(lines, isNotEmpty);
    for (final l in lines) {
      expect(jsonDecode(l), isA<Map<String, dynamic>>());
    }
  }, timeout: const Timeout(Duration(minutes: 2)));

  // a directory that cannot be read does not stop the extraction: it is a load error, and the rest is read
  test('a directory that cannot be read is said, and the rest is read', () async {
    if (Platform.isWindows || (await Process.run('id', ['-u'])).stdout.toString().trim() == '0') {
      return; // the permission of a directory is not kept from the root user
    }
    final dir = Directory.systemTemp.createTempSync('rivus-dart-unreadable');
    final locked = Directory(p.join(dir.path, 'locked'))..createSync();
    try {
      File(p.join(dir.path, 'pubspec.yaml')).writeAsStringSync('name: tmp_pkg\nenvironment:\n  sdk: ^3.0.0\n');
      Directory(p.join(dir.path, 'lib')).createSync();
      File(p.join(dir.path, 'lib', 'a.dart')).writeAsStringSync('int f(int x) => x;\n');
      File(p.join(locked.path, 'b.dart')).writeAsStringSync('int g(int x) => x;\n');
      await Process.run('chmod', ['000', locked.path]);
      final out = StringBuffer();
      await extract(dir.path, out, deps: false);
      final lines = const LineSplitter().convert(out.toString()).map((l) => jsonDecode(l) as Map<String, dynamic>).toList();
      final errs = lines.where((m) => m['kind'] == 'load_error').map((m) => m['load_error'] as Map<String, dynamic>).toList();
      expect(errs.any((e) => e['package'] == 'locked' && (e['message'] as String).contains('could not be read') && e['unread'] == true), isTrue,
          reason: 'load errors: $errs');
      expect(lines.any((m) => m['kind'] == 'func'), isTrue, reason: 'the readable file is still read');
    } finally {
      await Process.run('chmod', ['755', locked.path]);
      dir.deleteSync(recursive: true);
    }
  }, timeout: const Timeout(Duration(minutes: 2)));

  // a function that cannot be lowered is said as `fn_skipped` (the one record that the core turns into the note "the body was there": the
  // Rust extractor says it so), with the reason in one line: no stack trace, which holds the paths of this machine, in the stream
  test('a function that fails is said as fn_skipped, without a stack trace', () async {
    final dir = Directory.systemTemp.createTempSync('rivus-dart-skipped');
    try {
      File(p.join(dir.path, 'pubspec.yaml')).writeAsStringSync('name: tmp_pkg\nenvironment:\n  sdk: ^3.0.0\n');
      Directory(p.join(dir.path, 'lib')).createSync();
      File(p.join(dir.path, 'lib', 'a.dart')).writeAsStringSync('int keepMe(int x) => x;\nint failMe(int x) => x + 1;\n');
      // the package is resolved (no dependency: nothing is fetched), so that the IDs are `package:tmp_pkg/a.dart#...`, not paths of this machine
      final pub = await Process.run(Platform.resolvedExecutable, ['pub', 'get', '--offline'], workingDirectory: dir.path);
      expect(pub.exitCode, 0, reason: '${pub.stdout}${pub.stderr}');
      final out = StringBuffer();
      await extract(dir.path, out, deps: false, failIn: 'failMe');
      final lines = const LineSplitter().convert(out.toString()).map((l) => jsonDecode(l) as Map<String, dynamic>).toList();
      final skipped = lines.where((m) => m['kind'] == 'fn_skipped').map((m) => m['fn_skipped'] as Map<String, dynamic>).toList();
      expect(skipped, hasLength(1), reason: 'records: ${lines.map((m) => m['kind']).toList()}');
      expect(skipped.single['func'], contains('failMe'));
      expect(skipped.single['unit'], 'package:tmp_pkg');
      final why = skipped.single['why'] as String;
      expect(why, contains('injected failure'));
      expect(why, isNot(contains('\n')));
      expect(why, isNot(contains(dir.path)), reason: 'a path of this machine is in the stream: $why');
      expect(why, isNot(contains('#0')), reason: 'a stack trace is in the stream: $why');
      expect(lines.where((m) => m['kind'] == 'load_error'), isEmpty, reason: 'the function is said once, as fn_skipped');
      final funcs = lines.where((m) => m['kind'] == 'func').map((m) => (m['func'] as Map<String, dynamic>)['id'] as String).toList();
      expect(funcs.any((id) => id.contains('keepMe')), isTrue, reason: 'the others are read: $funcs');
      expect(funcs.any((id) => id.contains('failMe')), isFalse);
    } finally {
      dir.deleteSync(recursive: true);
    }
  }, timeout: const Timeout(Duration(minutes: 2)));
}
