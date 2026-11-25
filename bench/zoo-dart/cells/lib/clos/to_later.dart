// @cell(F/clos/to-later)
// @shape: a closure that captures a value is given to a function of the core library that calls it later (Future.catchError, Future.whenComplete, Stream.map, Stream.listen)
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-later/vuln', vuln);
  zooCase('F/clos/to-later/vuln2', vuln2);
  zooCase('F/clos/to-later/vuln3', vuln3);
  zooCase('F/clos/to-later/vuln4', vuln4);
  zooCase('F/clos/to-later/fixed', fixed);
  zooCase('F/clos/to-later/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final v = src('clos-to-later-vuln'); // @source(clos-to-later-vuln)
  await Future<void>.value().whenComplete(() {
    sinkCmd(v); // @sink(clos-to-later-vuln)
  });
}

// vuln2: the error of the future is handled by the function.
Future<void> vuln2() async { // @vuln
  final v = src('clos-to-later-vuln2'); // @source(clos-to-later-vuln2)
  await Future<int>.error(StateError('x')).catchError((Object e) {
    sinkCmd(v); // @sink(clos-to-later-vuln2)
    return 0;
  });
}

// vuln3: the stream calls the function for each element.
Future<void> vuln3() async { // @vuln
  final v = src('clos-to-later-vuln3'); // @source(clos-to-later-vuln3)
  await Stream<int>.value(1).map((x) {
    sinkCmd(v); // @sink(clos-to-later-vuln3)
    return x;
  }).toList();
}

// vuln4: the listener gets the element.
Future<void> vuln4() async { // @vuln
  final v = src('clos-to-later-vuln4'); // @source(clos-to-later-vuln4)
  await Stream<String>.value(v).listen((s) {
    sinkCmd(s); // @sink(clos-to-later-vuln4)
  }).asFuture<void>();
}

Future<void> fixed() async { // @fixed
  final v = cleanCmd(src('clos-to-later-fixed'));
  await Future<void>.value().whenComplete(() {
    sinkCmd(v); // @sink(none)
  });
  await Stream<String>.value(v).listen((s) {
    sinkCmd(s); // @sink(none)
  }).asFuture<void>();
}

Future<void> decoy() async { // @decoy
  final a = src('clos-to-later-decoy'); // @source(clos-to-later-decoy)
  const k = 'constant';
  await Future<void>.value().whenComplete(() {
    sinkCmd(k); // @sink(none) @nosink(clos-to-later-decoy)
  });
  await Future<int>.error(StateError('x')).catchError((Object e) {
    sinkCmd(k); // @sink(none) @nosink(clos-to-later-decoy)
    return 0;
  });
  await Stream<String>.value(k).listen((s) {
    sinkCmd(s); // @sink(none) @nosink(clos-to-later-decoy)
  }).asFuture<void>();
  sinkSql(a); // @sink(clos-to-later-decoy)
}
