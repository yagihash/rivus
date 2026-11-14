import 'dart:io';

import 'package:test/test.dart';

void main() {
  late String out;

  setUpAll(() async {
    final r = await Process.run(Platform.resolvedExecutable, ['run', 'bin/resolve.dart']);
    expect(r.exitCode, 0, reason: '${r.stderr}');
    out = r.stdout as String;
  });

  test('a monorepo gets one context per package, and one for its root', () {
    expect(out, contains('contexts: [mono, a, b]'));
  });

  test('a call in package b resolves to its declaration in package a', () {
    expect(out, matches(RegExp(r'b: readQuery -> readQuery declared in .*/samples/mono/a/lib/a\.dart')));
    expect(out, matches(RegExp(r'b: parseUser -> parseUser declared in .*/samples/mono/a/lib/a\.dart')));
  });

  test('a call into generated code resolves to the generated part file', () {
    expect(out, matches(RegExp(r'user\.dart: _\$UserFromJson -> .* declared in .*/a/lib/user\.g\.dart')));
    expect(out, matches(RegExp(r'user\.dart: _\$UserToJson -> .* declared in .*/a/lib/user\.g\.dart')));
  });

  test('none of it has compile time errors', () {
    expect(out, contains('errors: []'));
  });
}
