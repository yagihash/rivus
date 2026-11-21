// @cell(F/clos/two-callers)
// @shape: one helper of the project calls the function it is given with a value; two callers give it their own closures
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/two-callers/vuln', vuln);
  zooCase('F/clos/two-callers/fixed', fixed);
  zooCase('F/clos/two-callers/decoy', decoy);
}

void applyVuln(void Function(String) f, String s) => f(s);

void vuln() { // @vuln
  final v = src('clos-two-callers-vuln'); // @source(clos-two-callers-vuln)
  applyVuln((x) {
    sinkCmd(x); // @sink(clos-two-callers-vuln)
  }, v);
}

void applyFixed(void Function(String) f, String s) => f(s);

void fixed() { // @fixed
  final v = cleanCmd(src('clos-two-callers-fixed'));
  applyFixed((x) {
    sinkCmd(x); // @sink(none)
  }, v);
}

void applyDecoy(void Function(String) f, String s) => f(s);

void decoyA(String s) {
  applyDecoy((x) {
    sinkCmd(x); // @sink(none) @nosink(clos-two-callers-decoy)
  }, s);
}

void decoyB(String s) {
  applyDecoy((x) {
    sinkSql(x); // @sink(clos-two-callers-decoy)
  }, s);
}

void decoy() { // @decoy
  final v = src('clos-two-callers-decoy'); // @source(clos-two-callers-decoy)
  decoyA('constant');
  decoyB(v);
}
