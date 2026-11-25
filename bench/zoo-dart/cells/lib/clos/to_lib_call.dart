// @cell(F/clos/to-lib-call)
// @shape: a closure that captures a value is given to a method of the core library that calls it with what the receiver holds (Iterable.fold: the start value and the element), and the closure runs in that call
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-lib-call/vuln', vuln);
  zooCase('F/clos/to-lib-call/vuln2', vuln2);
  zooCase('F/clos/to-lib-call/fixed', fixed);
  zooCase('F/clos/to-lib-call/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('clos-to-lib-call-vuln'); // @source(clos-to-lib-call-vuln)
  [1].fold<int>(0, (a, _) {
    sinkCmd(v); // @sink(clos-to-lib-call-vuln)
    return a;
  });
}

// vuln2: the element is the second argument of the function.
void vuln2() { // @vuln
  final v = src('clos-to-lib-call-vuln2'); // @source(clos-to-lib-call-vuln2)
  <String>[v].fold<int>(0, (a, e) {
    sinkCmd(e); // @sink(clos-to-lib-call-vuln2)
    return a;
  });
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-to-lib-call-fixed'));
  [1].fold<int>(0, (a, _) {
    sinkCmd(v); // @sink(none)
    return a;
  });
  <String>[v].fold<int>(0, (a, e) {
    sinkCmd(e); // @sink(none)
    return a;
  });
}

void decoy() { // @decoy
  final v = src('clos-to-lib-call-decoy'); // @source(clos-to-lib-call-decoy)
  const k = 'constant';
  [1].fold<int>(0, (a, _) {
    sinkCmd(k); // @sink(none) @nosink(clos-to-lib-call-decoy)
    return a;
  });
  <String>[k].fold<int>(0, (a, e) {
    sinkCmd(e); // @sink(none) @nosink(clos-to-lib-call-decoy)
    return a;
  });
  sinkSql(v); // @sink(clos-to-lib-call-decoy)
}
