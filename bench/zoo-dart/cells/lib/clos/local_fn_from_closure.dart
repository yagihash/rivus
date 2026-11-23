// @cell(F/clos/local-fn-from-closure)
// @shape: a local function declared in a function is called from a closure (a callback) in the same function, and the value reaches the sink only through the local function
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/local-fn-from-closure/vuln', vuln);
  zooCase('F/clos/local-fn-from-closure/fixed', fixed);
  zooCase('F/clos/local-fn-from-closure/decoy', decoy);
}

void vuln() { // @vuln
  void emit(String s) {
    sinkCmd(s); // @sink(clos-local-fn-from-closure-vuln)
  }

  final v = src('clos-local-fn-from-closure-vuln'); // @source(clos-local-fn-from-closure-vuln) @miss(silent) @ref(#99)
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

  final v = src('clos-local-fn-from-closure-decoy'); // @source(clos-local-fn-from-closure-decoy) @miss(silent) @ref(#99)
  [1].forEach((_) {
    emit(v, 'constant');
  });
}
