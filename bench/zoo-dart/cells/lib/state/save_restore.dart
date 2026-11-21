// @cell(F/state/save-restore)
// @shape: a function saves a field, sets it, calls the work and restores it; the decoy reads another field
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/save-restore/vuln', vuln);
  zooCase('F/state/save-restore/fixed', fixed);
  zooCase('F/state/save-restore/decoy', decoy);
}

class State {
  String a = '';
  String b = '';
}

final sVuln = State();

void readVuln() {
  sinkCmd(sVuln.a); // @sink(state-save-restore-vuln)
}

void vuln() { // @vuln
  final old = sVuln.a;
  sVuln.a = src('state-save-restore-vuln'); // @source(state-save-restore-vuln)
  readVuln();
  sVuln.a = old;
}

final sFixed = State();

void readFixed() {
  sinkCmd(sFixed.a); // @sink(none)
}

void fixed() { // @fixed
  final old = sFixed.a;
  sFixed.a = cleanCmd(src('state-save-restore-fixed'));
  readFixed();
  sFixed.a = old;
}

final sDecoy = State();

void readDecoyB() {
  sinkCmd(sDecoy.b); // @sink(none) @nosink(state-save-restore-decoy)
}

void readDecoyA() {
  sinkSql(sDecoy.a); // @sink(state-save-restore-decoy)
}

void decoy() { // @decoy
  final old = sDecoy.a;
  sDecoy.a = src('state-save-restore-decoy'); // @source(state-save-restore-decoy)
  sDecoy.b = 'constant';
  readDecoyB();
  readDecoyA();
  sDecoy.a = old;
}
