import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rivus_dart/extract.dart';
import 'package:test/test.dart';

void main() {
  late Directory root;
  setUp(() => root = Directory.systemTemp.createTempSync('rivus-dart-pkgdir'));
  tearDown(() => root.deleteSync(recursive: true));

  test('a directory named lib inside the lib of a package is not taken for the package', () {
    final pkg = p.join(root.path, 'pkg');
    Directory(p.join(pkg, 'lib', 'src', 'lib')).createSync(recursive: true);
    File(p.join(pkg, 'pubspec.yaml')).writeAsStringSync('name: pkg\n');
    expect(packageDirOfFile(p.join(pkg, 'lib', 'a.dart')), pkg);
    expect(packageDirOfFile(p.join(pkg, 'lib', 'src', 'lib', 'b.dart')), pkg);
  });

  test('without a pubspec.yaml, the directory above the last lib is the package', () {
    final pkg = p.join(root.path, 'pkg');
    expect(packageDirOfFile(p.join(pkg, 'lib', 'src', 'a.dart')), pkg);
    expect(packageDirOfFile(p.join(pkg, 'tool', 'a.dart')), p.join(pkg, 'tool'));
  });
}
