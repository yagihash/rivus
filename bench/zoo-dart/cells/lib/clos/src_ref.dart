// @cell(F/clos/src-ref)
// @shape: a function that the rules declare a source, named as a value (not written as a closure), is given to a function of the core library that calls it and puts what it returns in its result: the call is the place of the source
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/src-ref/vuln', vuln);
  zooCase('F/clos/src-ref/fixed', fixed);
  zooCase('F/clos/src-ref/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  {
    final t = <String>['clos-src-ref-vuln-map'].map(src).first; // @source(clos-src-ref-vuln-map)
    sinkCmd(t); // @sink(clos-src-ref-vuln-map)
  }
  {
    final t = await Future<String>.value('clos-src-ref-vuln-then').then(src); // @source(clos-src-ref-vuln-then)
    sinkCmd(t); // @sink(clos-src-ref-vuln-then)
  }
  {
    final f = src;
    final t = <String>['clos-src-ref-vuln-variable'].map(f).first; // @source(clos-src-ref-vuln-variable)
    sinkCmd(t); // @sink(clos-src-ref-vuln-variable)
  }
  {
    // written as a closure: the source is the call in it, and the call of the library has none of its own
    final t = <String>['x'].map((e) {
      return src('clos-src-ref-vuln-closure'); // @source(clos-src-ref-vuln-closure)
    }).first;
    sinkCmd(t); // @sink(clos-src-ref-vuln-closure)
  }
}

Future<void> fixed() async { // @fixed
  final t = cleanCmd(<String>['clos-src-ref-fixed-map'].map(src).first);
  sinkCmd(t); // @sink(none)
  final u = cleanCmd(await Future<String>.value('clos-src-ref-fixed-then').then(src));
  sinkCmd(u); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final t = src('clos-src-ref-decoy'); // @source(clos-src-ref-decoy)
  // a function that is no source: what it returns is not tainted
  final a = <String>['constant'].map(plain).first;
  sinkCmd(a); // @sink(none) @nosink(clos-src-ref-decoy)
  // a source that the call puts nowhere (the result of the call is a bool)
  final b = <String>['constant'].any(srcB);
  sinkCmd('$b'); // @sink(none) @nosink(clos-src-ref-decoy)
  sinkSql(t); // @sink(clos-src-ref-decoy)
}

String plain(String s) => '$s!';
