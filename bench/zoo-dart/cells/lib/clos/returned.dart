// @cell(F/clos/returned)
// @shape: a function returns a closure that captures its parameter, and the caller calls it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/returned/vuln', vuln);
  zooCase('F/clos/returned/fixed', fixed);
  zooCase('F/clos/returned/decoy', decoy);
}

void Function() make1(String s) {
  return () {
    sinkCmd(s); // @sink(clos-returned-vuln)
  };
}

void vuln() { // @vuln
  make1(src('clos-returned-vuln'))(); // @source(clos-returned-vuln)
}

void Function() makeFixed(String s) {
  return () {
    sinkCmd(s); // @sink(none)
  };
}

void fixed() { // @fixed
  makeFixed(cleanCmd(src('clos-returned-fixed')))();
}

void Function() makeDecoy(String a, String b) {
  return () {
    sinkCmd(b); // @sink(none) @nosink(clos-returned-decoy)
    sinkSql(a); // @sink(clos-returned-decoy)
  };
}

void decoy() { // @decoy
  makeDecoy(src('clos-returned-decoy'), 'constant')(); // @source(clos-returned-decoy)
}
