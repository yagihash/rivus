// @cell(F/clos/two-callers)
// @shape: one helper calls the closure that it is given, with the value that it is given; two callers give it different closures (the decoy's other caller must not get the first one's value)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/two-callers/vuln', vuln);
  zooCase('F/clos/two-callers/fixed', fixed);
  zooCase('F/clos/two-callers/decoy', decoy);
}

void each(String x, void Function(String) f) => f(x);

void vuln() { // @vuln
  final a = src('clos-two-callers-vuln'); // @source(clos-two-callers-vuln)
  each(a, (s) => sinkCmd(s)); // @sink(clos-two-callers-vuln)
}

void eachFixed(String x, void Function(String) f) => f(x);

void fixed() { // @fixed
  final a = cleanCmd(src('clos-two-callers-fixed'));
  eachFixed(a, (s) => sinkCmd(s)); // @sink(none)
}

void eachDecoy(String x, void Function(String) f) => f(x);

void decoy() { // @decoy
  final a = src('clos-two-callers-decoy'); // @source(clos-two-callers-decoy)
  eachDecoy(a, (s) => sinkSql(s)); // @sink(clos-two-callers-decoy)
  eachDecoy('constant', (s) => sinkCmd(s)); // @sink(none) @nosink(clos-two-callers-decoy)
}
