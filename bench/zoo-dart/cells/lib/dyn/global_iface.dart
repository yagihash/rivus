// @cell(F/dyn/global-iface)
// @shape: a top-level variable of an interface type holds an object that holds the value, and another function reads it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/global-iface/vuln', vuln);
  zooCase('F/dyn/global-iface/fixed', fixed);
  zooCase('F/dyn/global-iface/decoy', decoy);
}

abstract class Getter {
  String get();
}

class Box implements Getter {
  Box(this.v);
  final String v;

  @override
  String get() => v;
}

Getter? gVuln;

void readVuln() {
  sinkCmd(gVuln!.get()); // @sink(dyn-global-iface-vuln)
}

void vuln() { // @vuln
  gVuln = Box(src('dyn-global-iface-vuln')); // @source(dyn-global-iface-vuln)
  readVuln();
}

Getter? gFixed;

void readFixed() {
  sinkCmd(gFixed!.get()); // @sink(none)
}

void fixed() { // @fixed
  gFixed = Box(cleanCmd(src('dyn-global-iface-fixed')));
  readFixed();
}

Getter? gDecoyA;
Getter? gDecoyB;

void readDecoy() {
  sinkCmd(gDecoyB!.get()); // @sink(none) @nosink(dyn-global-iface-decoy)
  sinkSql(gDecoyA!.get()); // @sink(dyn-global-iface-decoy)
}

void decoy() { // @decoy
  gDecoyA = Box(src('dyn-global-iface-decoy')); // @source(dyn-global-iface-decoy)
  gDecoyB = Box('constant');
  readDecoy();
}
