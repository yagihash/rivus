// @cell(F/state/global-direct)
// @shape: a function writes a top-level variable and another function reads it (the decoy has two variables)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/global-direct/vuln', vuln);
  zooCase('F/state/global-direct/fixed', fixed);
  zooCase('F/state/global-direct/decoy', decoy);
}

String gVuln = '';

void readVuln() {
  sinkCmd(gVuln); // @sink(state-global-direct-vuln)
}

void vuln() { // @vuln
  gVuln = src('state-global-direct-vuln'); // @source(state-global-direct-vuln)
  readVuln();
}

String gFixed = '';

void readFixed() {
  sinkCmd(gFixed); // @sink(none)
}

void fixed() { // @fixed
  gFixed = cleanCmd(src('state-global-direct-fixed'));
  readFixed();
}

String gDecoyA = '';
String gDecoyB = '';

void readDecoy() {
  sinkCmd(gDecoyB); // @sink(none) @nosink(state-global-direct-decoy)
  sinkSql(gDecoyA); // @sink(state-global-direct-decoy)
}

void decoy() { // @decoy
  gDecoyA = src('state-global-direct-decoy'); // @source(state-global-direct-decoy)
  gDecoyB = 'constant';
  readDecoy();
}
