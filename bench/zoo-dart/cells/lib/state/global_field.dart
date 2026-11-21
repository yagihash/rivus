// @cell(F/state/global-field)
// @shape: a function writes one field of a top-level variable that is an object, and another function reads it; the decoy reads another field
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/global-field/vuln', vuln);
  zooCase('F/state/global-field/fixed', fixed);
  zooCase('F/state/global-field/decoy', decoy);
}

class Conf {
  String a = '';
  String b = '';
}

final gVuln = Conf();

void readVuln() {
  sinkCmd(gVuln.a); // @sink(state-global-field-vuln)
}

void vuln() { // @vuln
  gVuln.a = src('state-global-field-vuln'); // @source(state-global-field-vuln)
  readVuln();
}

final gFixed = Conf();

void readFixed() {
  sinkCmd(gFixed.a); // @sink(none)
}

void fixed() { // @fixed
  gFixed.a = cleanCmd(src('state-global-field-fixed'));
  readFixed();
}

final gDecoy = Conf();

void readDecoy() {
  sinkCmd(gDecoy.b); // @sink(none) @nosink(state-global-field-decoy)
  sinkSql(gDecoy.a); // @sink(state-global-field-decoy)
}

void decoy() { // @decoy
  gDecoy.a = src('state-global-field-decoy'); // @source(state-global-field-decoy)
  gDecoy.b = 'constant';
  readDecoy();
}
