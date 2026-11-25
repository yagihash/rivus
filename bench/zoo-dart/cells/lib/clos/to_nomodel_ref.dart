// @cell(F/clos/to-nomodel-ref)
// @shape: a function that the rules declare a sink, named as a value (not written as a closure), is given to a function of the core library that has no row of calls: the call is the place of the sink
// The function that this cell gives a function to (Zone.runUnary) must have no `calls:` row. When a row is added for it, move the case to another function that has none.
import 'dart:async';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-nomodel-ref/vuln', vuln);
  zooCase('F/clos/to-nomodel-ref/fixed', fixed);
  zooCase('F/clos/to-nomodel-ref/decoy', decoy);
}

void vuln() { // @vuln
  {
    final t = src('clos-to-nomodel-ref-vuln-direct'); // @source(clos-to-nomodel-ref-vuln-direct)
    Zone.current.runUnary(sinkCmd, t); // @sink(clos-to-nomodel-ref-vuln-direct)
  }
  {
    final t = src('clos-to-nomodel-ref-vuln-variable'); // @source(clos-to-nomodel-ref-vuln-variable)
    final f = sinkCmd;
    Zone.current.runUnary(f, t); // @sink(clos-to-nomodel-ref-vuln-variable)
  }
  {
    // written as a closure: the sink is the call in it, and the call of the library has none of its own
    final t = src('clos-to-nomodel-ref-vuln-closure'); // @source(clos-to-nomodel-ref-vuln-closure)
    Zone.current.runUnary((String e) {
      sinkCmd(e); // @sink(clos-to-nomodel-ref-vuln-closure)
    }, t);
  }
}

void fixed() { // @fixed
  final t = cleanCmd(src('clos-to-nomodel-ref-fixed'));
  Zone.current.runUnary(sinkCmd, t); // @sink(none)
  final f = sinkCmd;
  Zone.current.runUnary(f, t); // @sink(none)
}

void decoy() { // @decoy
  final t = src('clos-to-nomodel-ref-decoy'); // @source(clos-to-nomodel-ref-decoy)
  // the argument is a constant: the value of the source is not what the library hands to the function
  Zone.current.runUnary(sinkCmd, 'constant'); // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
  final f = sinkCmd;
  Zone.current.runUnary(f, 'constant'); // @sink(none) @nosink(clos-to-nomodel-ref-decoy)
  // a function that is no sink, given the tainted value
  Zone.current.runUnary(isEmpty, t);
  sinkSql(t); // @sink(clos-to-nomodel-ref-decoy)
}

void isEmpty(String s) {}
