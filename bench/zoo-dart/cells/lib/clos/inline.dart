// @cell(F/clos/inline)
// @shape: a closure that captures a value is called where it is made
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/inline/vuln', vuln);
  zooCase('F/clos/inline/fixed', fixed);
  zooCase('F/clos/inline/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('clos-inline-vuln'); // @source(clos-inline-vuln)
  () {
    sinkCmd(v); // @sink(clos-inline-vuln)
  }();
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-inline-fixed'));
  () {
    sinkCmd(v); // @sink(none)
  }();
}

void decoy() { // @decoy
  final v = src('clos-inline-decoy'); // @source(clos-inline-decoy)
  const k = 'constant';
  () {
    sinkCmd(k); // @sink(none) @nosink(clos-inline-decoy)
  }();
  () {
    sinkSql(v); // @sink(clos-inline-decoy)
  }();
}
