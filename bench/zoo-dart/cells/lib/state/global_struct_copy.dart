// @cell(F/state/global-struct-copy)
// @shape: a function copies a top-level object whole (a copy constructor), and reads a field of the copy; the decoy reads another field
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/global-struct-copy/vuln', vuln);
  zooCase('F/state/global-struct-copy/fixed', fixed);
  zooCase('F/state/global-struct-copy/decoy', decoy);
}

class Conf {
  Conf();
  Conf.copy(Conf o)
      : a = o.a,
        b = o.b;
  String a = '';
  String b = '';
}

final gVuln = Conf();

void readVuln() {
  final c = Conf.copy(gVuln);
  sinkCmd(c.a); // @sink(state-global-struct-copy-vuln)
}

void vuln() { // @vuln
  gVuln.a = src('state-global-struct-copy-vuln'); // @source(state-global-struct-copy-vuln)
  readVuln();
}

final gFixed = Conf();

void readFixed() {
  final c = Conf.copy(gFixed);
  sinkCmd(c.a); // @sink(none)
}

void fixed() { // @fixed
  gFixed.a = cleanCmd(src('state-global-struct-copy-fixed'));
  readFixed();
}

final gDecoy = Conf();

void readDecoy() {
  final c = Conf.copy(gDecoy);
  sinkCmd(c.b); // @sink(none) @nosink(state-global-struct-copy-decoy)
  sinkSql(c.a); // @sink(state-global-struct-copy-decoy)
}

void decoy() { // @decoy
  gDecoy.a = src('state-global-struct-copy-decoy'); // @source(state-global-struct-copy-decoy)
  gDecoy.b = 'constant';
  readDecoy();
}
