// @cell(F/dyn/xtalk-n-by-m)
// @shape: several handlers take the same parameter type (a source), and each has its own sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/xtalk-n-by-m/vuln', vuln);
  zooCase('F/dyn/xtalk-n-by-m/fixed', fixed);
  zooCase('F/dyn/xtalk-n-by-m/decoy', decoy);
}

void handler(In i) { // @source(dyn-xtalk-n-by-m-vuln)
  sinkCmd(i.value); // @sink(dyn-xtalk-n-by-m-vuln)
}

void vuln() { // @vuln
  handler(In('dyn-xtalk-n-by-m-vuln'));
}

void handlerFixed(In i) {
  sinkCmd(cleanCmd(i.value)); // @sink(none)
}

void fixed() { // @fixed
  handlerFixed(In('dyn-xtalk-n-by-m-fixed'));
}

// three handlers of one parameter type: each source reaches its own sink and no other
void handlerA(In i) { // @source(dyn-xtalk-n-by-m-decoy)
  sinkCmd(i.value); // @sink(dyn-xtalk-n-by-m-decoy)
}

void handlerB(In i) { // @source(dyn-xtalk-n-by-m-decoy-b)
  sinkSql(i.value); // @sink(dyn-xtalk-n-by-m-decoy-b)
}

void handlerC(In i) {
  sinkCmd('constant'); // @sink(none) @nosink(dyn-xtalk-n-by-m-decoy) @nosink(dyn-xtalk-n-by-m-decoy-b)
}

void decoy() { // @decoy
  handlerA(In('dyn-xtalk-n-by-m-decoy'));
  handlerB(In('dyn-xtalk-n-by-m-decoy-b'));
  handlerC(In('dyn-xtalk-n-by-m-decoy-c'));
}
