// @cell(F/dyn/sink-as-value)
// @shape: a function that a rule names as a sink is made a function value (final f = sinkCmd) and the value is called
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-as-value/vuln', vuln);
  zooCase('F/dyn/sink-as-value/fixed', fixed);
  zooCase('F/dyn/sink-as-value/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('dyn-sink-as-value-vuln'); // @source(dyn-sink-as-value-vuln)
  final f = sinkCmd;
  f(v); // @sink(dyn-sink-as-value-vuln)
}

void fixed() { // @fixed
  final v = cleanCmd(src('dyn-sink-as-value-fixed'));
  final f = sinkCmd;
  f(v); // @sink(none)
}

void decoy() { // @decoy
  final v = src('dyn-sink-as-value-decoy'); // @source(dyn-sink-as-value-decoy)
  final f = sinkCmd;
  final g = sinkSql;
  f('constant'); // @sink(none) @nosink(dyn-sink-as-value-decoy)
  g(v); // @sink(dyn-sink-as-value-decoy)
}
