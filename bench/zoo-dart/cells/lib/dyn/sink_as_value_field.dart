// @cell(F/dyn/sink-as-value-field)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a field of a struct and called through the field
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-as-value-field/vuln', vuln);
  zooCase('F/dyn/sink-as-value-field/fixed', fixed);
  zooCase('F/dyn/sink-as-value-field/decoy', decoy);
}

class Holder {
  final void Function(String)? cmd;
  final void Function(String)? sql;
  Holder({this.cmd, this.sql});
}

void vuln() { // @vuln
  final v = src('dyn-sink-as-value-field-vuln'); // @source(dyn-sink-as-value-field-vuln) @miss(unresolved_callee) @ref(#124,PARITY)
  final h = Holder(cmd: sinkCmd);
  h.cmd!(v); // @sink(dyn-sink-as-value-field-vuln)
}

void fixed() { // @fixed
  final v = cleanCmd(src('dyn-sink-as-value-field-fixed'));
  final h = Holder(cmd: sinkCmd);
  h.cmd!(v); // @sink(none)
}

void decoy() { // @decoy
  final v = src('dyn-sink-as-value-field-decoy'); // @source(dyn-sink-as-value-field-decoy) @miss(unresolved_callee) @ref(#124,PARITY)
  final h = Holder(cmd: sinkCmd, sql: sinkSql);
  h.cmd!('constant'); // @sink(none) @nosink(dyn-sink-as-value-field-decoy)
  h.sql!(v); // @sink(dyn-sink-as-value-field-decoy)
}
