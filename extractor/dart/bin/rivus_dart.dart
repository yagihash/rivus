// rivus_dart [--no-deps] [directory]: writes the neutral IR of the Dart code under the
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
  final rest = args.where((a) => a != '--no-deps').toList();
  final root = rest.isEmpty ? '.' : rest.first;
  final sink = StringBuffer();
  await extract(root, sink, deps: deps);
  stdout.write(sink);
}
