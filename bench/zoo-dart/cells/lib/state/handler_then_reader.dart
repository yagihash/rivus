// @cell(F/state/handler-then-reader)
// @shape: one entry point (a handler) writes a top-level variable from its parameter, and another entry point reads it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/handler-then-reader/vuln', vuln);
  zooCase('F/state/handler-then-reader/fixed', fixed);
  zooCase('F/state/handler-then-reader/decoy', decoy);
}

String lastVuln = '';

void writeVuln(In req) { // @source(state-handler-then-reader-vuln)
  lastVuln = req.value;
}

void readVuln() {
  sinkCmd(lastVuln); // @sink(state-handler-then-reader-vuln)
}

void vuln() { // @vuln
  writeVuln(In('state-handler-then-reader-vuln'));
  readVuln();
}

String lastFixed = '';

void writeFixed(In req) {
  lastFixed = cleanCmd(req.value);
}

void readFixed() {
  sinkCmd(lastFixed); // @sink(none)
}

void fixed() { // @fixed
  writeFixed(In('state-handler-then-reader-fixed'));
  readFixed();
}

String lastDecoyA = '';
String lastDecoyB = '';

void writeDecoyA(In req) { // @source(state-handler-then-reader-decoy)
  lastDecoyA = req.value;
}

void writeDecoyB(String s) {
  lastDecoyB = s;
}

void readDecoy() {
  sinkCmd(lastDecoyB); // @sink(none) @nosink(state-handler-then-reader-decoy)
  sinkSql(lastDecoyA); // @sink(state-handler-then-reader-decoy)
}

void decoy() { // @decoy
  writeDecoyA(In('state-handler-then-reader-decoy'));
  writeDecoyB('constant');
  readDecoy();
}
