// Calls the functions of package:path that the built-in model has a row for, with a marker (the text in the
// environment variable ZOO_MARKER, a different one in each variant), and writes what came out as JSON:
// [{"symbol": ..., "part": ..., "text": ...}]. cmd/rivus TestZooPathDart runs it in two variants and holds the
// rows to it: a function whose row says that its arguments flow to its result has to give another result for
// another marker (a row that carries what the function does not carry makes false findings), and a function whose
// result is the same for both markers must not have such a row. The answer is the run, not the model.
//
// A part is the argument that has the marker; the others are fixed text. No file is made, nothing is read from
// the disk: package:path only works on text (the posix style, so that the result does not depend on the machine).
import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

List<Map<String, String>> observe(String m) {
  final posix = p.Context(style: p.Style.posix, current: '/work');
  final out = <Map<String, String>>[];
  void add(String symbol, String part, Object? v) => out.add({'symbol': symbol, 'part': part, 'text': '$v'});

  // join and joinAll: every part of the path is in the result
  add('package:path/path.dart#join', 'arg0', p.join('$m/a', 'b'));
  add('package:path/path.dart#join', 'arg1', p.join('a', '$m'));
  add('package:path/path.dart#joinAll', 'arg0', p.joinAll(['a', '$m', 'c']));
  add('package:path/path.dart#normalize', 'arg0', p.normalize('a/./$m/../b/$m'));
  add('package:path/path.dart#relative', 'arg0', p.relative('/work/$m/x', from: '/work'));
  add('package:path/path.dart#relative', 'from', p.relative('/work/x', from: '/work/$m'));
  add('package:path/path.dart#absolute', 'arg0', p.absolute('$m/x'));
  add('package:path/path.dart#toUri', 'arg0', p.toUri('/$m/x'));
  // the same through a Context (the style is explicit), which is how a library that takes a style calls them
  add('package:path/src/context.dart#Context.join', 'arg0', posix.join('$m/a', 'b'));
  add('package:path/src/context.dart#Context.normalize', 'arg0', posix.normalize('a/./$m'));
  add('package:path/src/context.dart#Context.relative', 'arg0', posix.relative('/work/$m/x', from: '/work'));
  add('package:path/src/context.dart#Context.absolute', 'arg0', posix.absolute('$m/x'));
  add('package:path/src/context.dart#Context.toUri', 'arg0', posix.toUri('/$m/x'));
  add('package:path/src/context.dart#Context.joinAll', 'arg0', posix.joinAll(['a', '$m']));
  return out;
}

void main() {
  final m = Platform.environment['ZOO_MARKER'] ?? 'MARK';
  print(jsonEncode(observe(m)));
}
