// @cell(F/clos/partial-callers)
// @shape: one helper calls the function it is given; one caller gives a closure made right there, another a function it reads from a field (a caller whose value cannot be followed)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/partial-callers/vuln', vuln);
  zooCase('F/clos/partial-callers/fixed', fixed);
  zooCase('F/clos/partial-callers/decoy', decoy);
}

class Holder {
  final void Function(String) cb;
  Holder(this.cb);
}

void applyVuln(void Function(String) f, String s) => f(s);

void vuln() { // @vuln
  final v = src('clos-partial-callers-vuln'); // @source(clos-partial-callers-vuln) @miss(silent) @ref(PARTIAL-CALLERS,PARITY)
  applyVuln((x) {}, 'constant');
  final h = Holder((x) {
    sinkCmd(x); // @sink(clos-partial-callers-vuln)
  });
  applyVuln(h.cb, v);
}

void applyFixed(void Function(String) f, String s) => f(s);

void fixed() { // @fixed
  final v = cleanCmd(src('clos-partial-callers-fixed'));
  applyFixed((x) {}, 'constant');
  final h = Holder((x) {
    sinkCmd(x); // @sink(none)
  });
  applyFixed(h.cb, v);
}

void applyDecoy(void Function(String) f, String s) => f(s);

void decoy() { // @decoy
  final v = src('clos-partial-callers-decoy'); // @source(clos-partial-callers-decoy)
  final h = Holder((x) {
    sinkCmd(x); // @sink(none)
  });
  applyDecoy(h.cb, 'constant');
  applyDecoy((x) {
    sinkSql(x); // @sink(clos-partial-callers-decoy)
  }, v);
}
