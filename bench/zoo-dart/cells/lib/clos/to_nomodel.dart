// @cell(F/clos/to-nomodel)
// @shape: a closure that captures a value is given to a function of the core library that has no row of calls, and the result of the call is used
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-nomodel/vuln', vuln);
  zooCase('F/clos/to-nomodel/fixed', fixed);
  zooCase('F/clos/to-nomodel/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('clos-to-nomodel-vuln'); // @source(clos-to-nomodel-vuln)
  final out = Zone.current.run(() => v);
  sinkCmd(out); // @sink(clos-to-nomodel-vuln)
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-to-nomodel-fixed'));
  final out = Zone.current.run(() => v);
  sinkCmd(out); // @sink(none)
}

void decoy() { // @decoy
  final v = src('clos-to-nomodel-decoy'); // @source(clos-to-nomodel-decoy)
  final out = Zone.current.run(() => 'constant');
  sinkCmd(out); // @sink(none) @nosink(clos-to-nomodel-decoy)
  sinkSql(v); // @sink(clos-to-nomodel-decoy)
}
