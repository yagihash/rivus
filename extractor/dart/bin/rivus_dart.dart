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
  final sink = StringBuffer();
  await extract(root, sink, deps: deps, allowDepsUnder: allow);
  stdout.write(sink);
}
