// @cell(F/dart/closure-to-sdk)
// @shape: a closure that captures a value is only handed to the SDK (Timer.run, scheduleMicrotask), which calls it later
// @na(go: a Go closure is handed to a goroutine or a library the same way, F/clos has the forms that Go has)
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/closure-to-sdk/vuln', vuln);
  zooCase('F/dart/closure-to-sdk/vuln2', vuln2);
  zooCase('F/dart/closure-to-sdk/fixed', fixed);
  zooCase('F/dart/closure-to-sdk/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final t = src('dart-closure-to-sdk-vuln'); // @source(dart-closure-to-sdk-vuln)
  final done = Completer<void>();
  Timer.run(() {
    sinkCmd(t); // @sink(dart-closure-to-sdk-vuln)
    done.complete();
  });
  await done.future;
}

Future<void> vuln2() async { // @vuln
  final t = src('dart-closure-to-sdk-vuln2'); // @source(dart-closure-to-sdk-vuln2)
  final done = Completer<void>();
  scheduleMicrotask(() {
    sinkCmd(t); // @sink(dart-closure-to-sdk-vuln2)
    done.complete();
  });
  await done.future;
}

Future<void> fixed() async { // @fixed
  final t = cleanCmd(src('dart-closure-to-sdk-fixed'));
  final done = Completer<void>();
  Timer.run(() {
    sinkCmd(t); // @sink(none)
    done.complete();
  });
  await done.future;
}

Future<void> decoy() async { // @decoy
  final t = src('dart-closure-to-sdk-decoy'); // @source(dart-closure-to-sdk-decoy)
  final done = Completer<void>();
  Timer.run(() {
    sinkCmd('constant'); // @sink(none) @nosink(dart-closure-to-sdk-decoy)
    sinkSql(t); // @sink(dart-closure-to-sdk-decoy)
    done.complete();
  });
  await done.future;
}
