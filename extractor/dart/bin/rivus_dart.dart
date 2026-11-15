// rivus_dart [directory]: writes the neutral IR of the Dart code under the
// directory (the current one by default) to the standard output, as JSON
// Lines. See docs/ARCHITECTURE.md 2.7.7.
import 'dart:io';

import 'package:rivus_dart/extract.dart';

Future<void> main(List<String> args) async {
  if (args.contains('--version')) {
    stdout.writeln('rivus_dart rivus.ir/0');
    return;
  }
  final root = args.isEmpty ? '.' : args.first;
  final sink = StringBuffer();
  await extract(root, sink);
  stdout.write(sink);
}
