// rivus_dart [--no-deps] [--include-tests] [--allow-deps-under DIR]... [--dep-dir DIR]... [--body-replaced PACKAGE]... [directory]: writes the neutral IR of the Dart code under the
// directory (the current one by default) to the standard output, as JSON
// Lines (--no-deps leaves out the dependencies). See docs/ARCHITECTURE.md 2.7.7.
import 'dart:io';

import 'package:rivus_dart/extract.dart';

Future<void> main(List<String> args) async {
  if (args.contains('--version')) {
    stdout.writeln('rivus_dart rivus.ir/1 source=${const String.fromEnvironment('RIVUS_SRC_HASH', defaultValue: 'unknown')}');
    return;
  }
  final deps = !args.contains('--no-deps');
  final includeTests = args.contains('--include-tests');
  // --allow-deps-under DIR (repeatable): the dependencies under DIR may be read, though the project does not hold them
  final allow = <String>[];
  // --body-replaced PACKAGE (repeatable): package:name whose models stand for its code: it is not resolved or lowered
  final replaced = <String>[];
  // --dep-dir DIR (repeatable): a directory of the project (relative to the root, or absolute) whose files are read as dependencies
  final depDirs = <String>[];
  final rest = <String>[];
  // An option that takes a value and is the last argument has none, and an option that is not known is not a directory:
  // both stop with the status 2 (the usage), not read as the directory to analyse.
  Never usage(String why) {
    stderr.writeln('rivus_dart: $why');
    stderr.writeln('usage: rivus_dart [--no-deps] [--include-tests] [--allow-deps-under DIR]... [--dep-dir DIR]... [--body-replaced PACKAGE]... [directory]');
    exit(2);
  }

  for (var i = 0; i < args.length; i++) {
    if (args[i] == '--no-deps' || args[i] == '--include-tests') continue;
    if (args[i] == '--allow-deps-under' || args[i] == '--dep-dir' || args[i] == '--body-replaced') {
      if (i + 1 >= args.length) usage('${args[i]} needs a value');
      final v = args[++i];
      if (args[i - 1] == '--allow-deps-under') {
        allow.add(v);
      } else if (args[i - 1] == '--dep-dir') {
        depDirs.add(v);
      } else {
        replaced.add(v);
      }
      continue;
    }
    if (args[i].startsWith('--')) usage('unknown option ${args[i]}');
    rest.add(args[i]);
  }
  final root = rest.isEmpty ? '.' : rest.first;
  final sink = _Stdout();
  await extract(root, sink, deps: deps, includeTests: includeTests, allowDepsUnder: allow, bodyReplaced: replaced, depDirs: depDirs, drain: sink.drain, failIn: Platform.environment['RIVUS_DART_FAIL_IN']); // RIVUS_DART_FAIL_IN: for the tests only (a function that fails)
  await sink.drain();
}

/// The stream goes to the standard output as it is made, a unit at a time (not all at the end).
class _Stdout implements StringSink {
  final _buffer = StringBuffer();

  @override
  void write(Object? object) => _buffer.write(object);
  @override
  void writeAll(Iterable<dynamic> objects, [String separator = '']) => _buffer.writeAll(objects, separator);
  @override
  void writeCharCode(int charCode) => _buffer.writeCharCode(charCode);
  @override
  void writeln([Object? object = '']) => _buffer.writeln(object);

  Future<void> drain() async {
    if (_buffer.isEmpty) return;
    stdout.write(_buffer);
    _buffer.clear();
    await stdout.flush();
  }
}
