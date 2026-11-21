// @cell(F/dyn/iface-assert-ptr-recv)
// @shape: an object is held as an Object, cast to an interface (as), and read through it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/iface-assert-ptr-recv/vuln', vuln);
  zooCase('F/dyn/iface-assert-ptr-recv/fixed', fixed);
  zooCase('F/dyn/iface-assert-ptr-recv/decoy', decoy);
}

abstract class Getter {
  String get();
}

class Box implements Getter {
  Box(this.v);
  String v;

  @override
  String get() => v;
}

void vuln() { // @vuln
  final Object a = Box(src('dyn-iface-assert-ptr-recv-vuln')); // @source(dyn-iface-assert-ptr-recv-vuln) @miss(silent) @ref(CAST,PARITY)
  final g = a as Getter;
  sinkCmd(g.get()); // @sink(dyn-iface-assert-ptr-recv-vuln)
}

void fixed() { // @fixed
  final Object a = Box(cleanCmd(src('dyn-iface-assert-ptr-recv-fixed')));
  final g = a as Getter;
  sinkCmd(g.get()); // @sink(none)
}

void decoy() { // @decoy
  final Object a = Box(src('dyn-iface-assert-ptr-recv-decoy')); // @source(dyn-iface-assert-ptr-recv-decoy) @miss(silent) @ref(CAST,PARITY)
  final Object b = Box('constant');
  sinkCmd((b as Getter).get()); // @sink(none) @nosink(dyn-iface-assert-ptr-recv-decoy)
  sinkSql((a as Getter).get()); // @sink(dyn-iface-assert-ptr-recv-decoy)
}
