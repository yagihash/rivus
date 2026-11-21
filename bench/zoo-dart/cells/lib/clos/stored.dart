// @cell(F/clos/stored)
// @shape: a closure that captures a value is stored (a field, a top-level variable, a list) and another function calls it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/stored/vuln', vuln);
  zooCase('F/clos/stored/vuln2', vuln2);
  zooCase('F/clos/stored/vuln3', vuln3);
  zooCase('F/clos/stored/fixed', fixed);
  zooCase('F/clos/stored/decoy', decoy);
}

class Holder {
  void Function()? a;
  void Function()? b;
}

void callA(Holder h) => h.a!();

// a field
void vuln() { // @vuln
  final v = src('clos-stored-vuln'); // @source(clos-stored-vuln)
  final h = Holder();
  h.a = () {
    sinkCmd(v); // @sink(clos-stored-vuln)
  };
  callA(h);
}

// a top-level variable
void Function()? gFn;

void callG() => gFn!();

void vuln2() { // @vuln
  final v = src('clos-stored-vuln2'); // @source(clos-stored-vuln2)
  gFn = () {
    sinkCmd(v); // @sink(clos-stored-vuln2)
  };
  callG();
}

// a list
void runAll(List<void Function()> fs) {
  for (final f in fs) {
    f();
  }
}

void vuln3() { // @vuln
  final v = src('clos-stored-vuln3'); // @source(clos-stored-vuln3)
  runAll([
    () {
      sinkCmd(v); // @sink(clos-stored-vuln3)
    }
  ]);
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-stored-fixed'));
  final h = Holder();
  h.a = () {
    sinkCmd(v); // @sink(none)
  };
  callA(h);
}

void callBoth(Holder h) {
  h.a!();
  h.b!();
}

void decoy() { // @decoy
  final v = src('clos-stored-decoy'); // @source(clos-stored-decoy)
  const k = 'constant';
  final h = Holder();
  h.a = () {
    sinkCmd(k); // @sink(none) @nosink(clos-stored-decoy)
  };
  h.b = () {
    sinkSql(v); // @sink(clos-stored-decoy)
  };
  callBoth(h);
}
