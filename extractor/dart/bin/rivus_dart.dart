// rivus_dart [--no-deps] [--allow-deps-under DIR]... [directory]: writes the neutral IR of the Dart code under the
// directory (the current one by default) to the standard output, as JSON
// Lines (--no-deps leaves out the dependencies). See docs/ARCHITECTURE.md 2.7.7.
import 'dart:io';

import 'package:rivus_dart/extract.dart';

Future<void> main(List<String> args) async {
  if (args.contains('--version')) {
    stdout.writeln('rivus_dart rivus.ir/0');
    return;
  }
  final deps = !args.contains('--no-deps');
  // --allow-deps-under DIR (repeatable): the dependencies under DIR may be read, though the project does not hold them
  final allow = <String>[];
  final rest = <String>[];
  for (var i = 0; i < args.length; i++) {
    if (args[i] == '--no-deps') continue;
    if (args[i] == '--allow-deps-under' && i + 1 < args.length) {
      allow.add(args[++i]);
      continue;
    }
    rest.add(args[i]);
  }
  final root = rest.isEmpty ? '.' : rest.first;
  final sink = _Stdout();
  await extract(root, sink, deps: deps, allowDepsUnder: allow, drain: sink.drain, failIn: Platform.environment['RIVUS_DART_FAIL_IN']); // RIVUS_DART_FAIL_IN: for the tests only (a function that fails)
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
