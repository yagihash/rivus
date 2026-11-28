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
}
