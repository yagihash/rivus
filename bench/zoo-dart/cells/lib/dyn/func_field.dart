// @cell(F/dyn/func-field)
// @shape: a function value kept in a field of an object is called; the object has another function field with another sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/func-field/vuln', vuln);
  zooCase('F/dyn/func-field/fixed', fixed);
  zooCase('F/dyn/func-field/decoy', decoy);
}

class Handlers {
  void Function(String)? cmd;
  void Function(String)? sql;
}

void vuln() { // @vuln
  final v = src('dyn-func-field-vuln'); // @source(dyn-func-field-vuln) @miss(unmodeled_library_call,unresolved_callee) @ref(FUNC-VALUE-MEMORY,PARITY)
  final h = Handlers();
  h.cmd = (x) {
    sinkCmd(x); // @sink(dyn-func-field-vuln)
  };
  h.cmd!(v);
}

void fixed() { // @fixed
  final v = cleanCmd(src('dyn-func-field-fixed'));
  final h = Handlers();
  h.cmd = (x) {
    sinkCmd(x); // @sink(none)
  };
  h.cmd!(v);
}

void decoy() { // @decoy
  final v = src('dyn-func-field-decoy'); // @source(dyn-func-field-decoy) @miss(unmodeled_library_call,unresolved_callee) @ref(FUNC-VALUE-MEMORY,PARITY)
  final h = Handlers();
  h.cmd = (x) {
    sinkCmd(x); // @sink(none) @nosink(dyn-func-field-decoy)
  };
  h.sql = (x) {
    sinkSql(x); // @sink(dyn-func-field-decoy)
  };
  h.cmd!('constant');
  h.sql!(v);
}
