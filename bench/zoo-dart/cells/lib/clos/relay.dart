// @cell(F/clos/relay)
// @shape: a function gives the function it was given to a helper that calls it (wrapper(g) calls run(g)); two callers give it their own closures
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/relay/vuln', vuln);
  zooCase('F/clos/relay/fixed', fixed);
  zooCase('F/clos/relay/decoy', decoy);
}

void runVuln(void Function(String) f, String s) => f(s);
void wrapperVuln(void Function(String) g, String s) => runVuln(g, s);

void vuln() { // @vuln
  final v = src('clos-relay-vuln'); // @source(clos-relay-vuln)
  wrapperVuln((x) {
    sinkCmd(x); // @sink(clos-relay-vuln)
  }, v);
}

void runFixed(void Function(String) f, String s) => f(s);
void wrapperFixed(void Function(String) g, String s) => runFixed(g, s);

void fixed() { // @fixed
  final v = cleanCmd(src('clos-relay-fixed'));
  wrapperFixed((x) {
    sinkCmd(x); // @sink(none)
  }, v);
}

void runDecoy(void Function(String) f, String s) => f(s);
void wrapperDecoy(void Function(String) g, String s) => runDecoy(g, s);

void decoyA(String s) {
  wrapperDecoy((x) {
    sinkCmd(x); // @sink(none) @fp @nosink(clos-relay-decoy) @fp @ref(XTALK)
  }, s);
}

void decoyB(String s) {
  wrapperDecoy((x) {
    sinkSql(x); // @sink(clos-relay-decoy)
  }, s);
}

void decoy() { // @decoy
  final v = src('clos-relay-decoy'); // @source(clos-relay-decoy)
  decoyA('constant');
  decoyB(v);
}
