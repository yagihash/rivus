import 'dart:convert';

import 'package:rivus_dart/extract.dart';
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
}
