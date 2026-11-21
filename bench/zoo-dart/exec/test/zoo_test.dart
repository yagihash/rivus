// The harness of the Dart zoo: one test per case. cmd/rivus TestZooExecDart runs it three times
// (ZOO_VARIANT=0, ZOO_VARIANT=1, ZOO_VARIANT=0 with ZOO_REVERSE=1, in different processes) with
// `dart test --coverage`, and reads what the sinks got from the file of ZOO_OUT.
import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';
import 'package:zoo_exec/cases.dart';
import 'package:zoo_mark/zoo_mark.dart' as mark;

void main() {
  registerAll();
  final reverse = Platform.environment['ZOO_REVERSE'] == '1';
  final ids = mark.caseIds();
  for (final id in reverse ? ids.reversed.toList() : ids) {
    test(id, () async {
      final before = mark.hits().length;
      await mark.runCase(id);
      expect(mark.hits().length, greaterThan(before), reason: 'the case has recorded no call of a sink');
    });
  }
  tearDownAll(() {
    final out = Platform.environment['ZOO_OUT'];
    if (out == null) return;
    File(out).writeAsStringSync(const JsonEncoder.withIndent(' ').convert({
      'variant': mark.variant,
      'reverse': reverse,
      'cases': ids,
      'hits': [for (final h in mark.hits()) h.toJson()],
    }));
  });
}
