// @cell(F/clos/local-fn-from-closure)
// @shape: a local function declared in a function is called from a closure (a callback) in the same function, and the value reaches the sink only through the local function
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/local-fn-from-closure/vuln', vuln);
  zooCase('F/clos/local-fn-from-closure/fixed', fixed);
  zooCase('F/clos/local-fn-from-closure/decoy', decoy);
  zooCase('F/clos/local-fn-from-closure/vuln3', vuln3);
  zooCase('F/clos/local-fn-from-closure/vuln2', vuln2);
}

void vuln() { // @vuln
  void emit(String s) {
    sinkCmd(s); // @sink(clos-local-fn-from-closure-vuln)
  }

  final v = src('clos-local-fn-from-closure-vuln'); // @source(clos-local-fn-from-closure-vuln)
  [1].forEach((_) {
    emit(v);
  });
}

void fixed() { // @fixed
  void emit(String s) {
    sinkCmd(s); // @sink(none)
  }

  final v = cleanCmd(src('clos-local-fn-from-closure-fixed'));
  [1].forEach((_) {
    emit(v);
  });
}

void decoy() { // @decoy
  void emit(String a, String b) {
    sinkCmd(b); // @sink(none) @nosink(clos-local-fn-from-closure-decoy)
    sinkSql(a); // @sink(clos-local-fn-from-closure-decoy)
  }

  final v = src('clos-local-fn-from-closure-decoy'); // @source(clos-local-fn-from-closure-decoy)
  [1].forEach((_) {
    emit(v, 'constant');
  });
}

void vuln3() { // @vuln
  // two local functions: the closure calls only one of them with the source; the other one is called with a constant
  void keep(String s) {
    sinkCmd(s); // @sink(none) @nosink(clos-local-fn-from-closure-vuln3)
  }

  void use(String s) {
    sinkSql(s); // @sink(clos-local-fn-from-closure-vuln3)
  }

  final v = src('clos-local-fn-from-closure-vuln3'); // @source(clos-local-fn-from-closure-vuln3)
  [1].forEach((_) {
    use(v);
  });
  [1].forEach((_) {
    keep('constant');
  });
}

void vuln2() { // @vuln
  // a variable that holds one local function and later another: the closure may call either
  void first(String s) {
    sinkSql(s); // @sink(none)
  }

  void second(String s) {
    sinkCmd(s); // @sink(clos-local-fn-from-closure-vuln2)
  }

  var f = first;
  f('constant');
  f = second;
  final v = src('clos-local-fn-from-closure-vuln2'); // @source(clos-local-fn-from-closure-vuln2) @miss(unresolved_callee) @ref(#99,PARITY)
  [1].forEach((_) {
    f(v);
  });
}
