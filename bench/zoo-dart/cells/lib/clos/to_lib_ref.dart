// @cell(F/clos/to-lib-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the core library that calls it with what the receiver holds: the call is the place of the sink
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-lib-ref/vuln', vuln);
  zooCase('F/clos/to-lib-ref/fixed', fixed);
  zooCase('F/clos/to-lib-ref/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  {
    final t = src('clos-to-lib-ref-vuln-foreach'); // @source(clos-to-lib-ref-vuln-foreach)
    <String>[t].forEach(sinkCmd); // @sink(clos-to-lib-ref-vuln-foreach)
  }
  {
    final t = src('clos-to-lib-ref-vuln-map'); // @source(clos-to-lib-ref-vuln-map)
    <String>[t].map(sinkCmd).toList(); // @sink(clos-to-lib-ref-vuln-map)
  }
  {
    final t = src('clos-to-lib-ref-vuln-where'); // @source(clos-to-lib-ref-vuln-where)
    <String>[t].where(sinkCmdB).toList(); // @sink(clos-to-lib-ref-vuln-where)
  }
  {
    final t = src('clos-to-lib-ref-vuln-then'); // @source(clos-to-lib-ref-vuln-then)
    await Future<String>.value(t).then(sinkCmd); // @sink(clos-to-lib-ref-vuln-then)
  }
  {
    final t = src('clos-to-lib-ref-vuln-variable'); // @source(clos-to-lib-ref-vuln-variable)
    final f = sinkCmd;
    <String>[t].forEach(f); // @sink(clos-to-lib-ref-vuln-variable)
  }
  {
    // written as a closure: the sink is the call in it, and the call of the library has none of its own
    final t = src('clos-to-lib-ref-vuln-closure'); // @source(clos-to-lib-ref-vuln-closure)
    <String>[t].forEach((e) {
      sinkCmd(e); // @sink(clos-to-lib-ref-vuln-closure)
    });
  }
}

Future<void> fixed() async { // @fixed
  final t = cleanCmd(src('clos-to-lib-ref-fixed'));
  <String>[t].forEach(sinkCmd); // @sink(none)
  <String>[t].map(sinkCmd).toList(); // @sink(none)
  <String>[t].where(sinkCmdB).toList(); // @sink(none)
  await Future<String>.value(t).then(sinkCmd); // @sink(none)
  final f = sinkCmd;
  <String>[t].forEach(f); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final t = src('clos-to-lib-ref-decoy'); // @source(clos-to-lib-ref-decoy)
  // what the receiver holds is a constant: the value of the source is not what the library hands to the function
  <String>['constant'].forEach(sinkCmd); // @sink(none) @nosink(clos-to-lib-ref-decoy)
  <String>['constant'].map(sinkCmd).toList(); // @sink(none) @nosink(clos-to-lib-ref-decoy)
  <String>['constant'].where(sinkCmdB).toList(); // @sink(none) @nosink(clos-to-lib-ref-decoy)
  await Future<String>.value('constant').then(sinkCmd); // @sink(none) @nosink(clos-to-lib-ref-decoy)
  final f = sinkCmd;
  <String>['constant'].forEach(f); // @sink(none) @nosink(clos-to-lib-ref-decoy)
  // a function that is no sink, given the tainted elements
  <String>[t].forEach(isEmpty);
  sinkSql(t); // @sink(clos-to-lib-ref-decoy)
}

void isEmpty(String s) {}
