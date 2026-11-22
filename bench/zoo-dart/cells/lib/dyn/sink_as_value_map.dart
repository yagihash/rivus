// @cell(F/dyn/sink-as-value-map)
// @shape: a function that a rule names as a sink is made a function value; the function value is stored in a map and called by a constant key
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-as-value-map/vuln', vuln);
  zooCase('F/dyn/sink-as-value-map/fixed', fixed);
  zooCase('F/dyn/sink-as-value-map/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('dyn-sink-as-value-map-vuln'); // @source(dyn-sink-as-value-map-vuln) @miss(unresolved_callee) @ref(#124,PARITY)
  final m = <String, void Function(String)>{'cmd': sinkCmd};
  m['cmd']!(v); // @sink(dyn-sink-as-value-map-vuln)
}

void fixed() { // @fixed
  final v = cleanCmd(src('dyn-sink-as-value-map-fixed'));
  final m = <String, void Function(String)>{'cmd': sinkCmd};
  m['cmd']!(v); // @sink(none)
}

void decoy() { // @decoy
  final v = src('dyn-sink-as-value-map-decoy'); // @source(dyn-sink-as-value-map-decoy) @miss(unresolved_callee) @ref(#124,PARITY)
  final m = <String, void Function(String)>{'cmd': sinkCmd, 'sql': sinkSql};
  m['cmd']!('constant'); // @sink(none) @nosink(dyn-sink-as-value-map-decoy)
  m['sql']!(v); // @sink(dyn-sink-as-value-map-decoy)
}
