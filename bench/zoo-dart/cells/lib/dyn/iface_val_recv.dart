// @cell(F/dyn/iface-val-recv)
// @shape: a value is held by an object whose class has a final field (the closest form of a value receiver), and is read through an interface
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/iface-val-recv/vuln', vuln);
  zooCase('F/dyn/iface-val-recv/fixed', fixed);
  zooCase('F/dyn/iface-val-recv/decoy', decoy);
}

abstract class Getter {
  String get();
}

class Box implements Getter {
  const Box(this.v);
  final String v;

  @override
  String get() => v;
}

void vuln() { // @vuln
  final Getter g = Box(src('dyn-iface-val-recv-vuln')); // @source(dyn-iface-val-recv-vuln)
  sinkCmd(g.get()); // @sink(dyn-iface-val-recv-vuln)
}

void fixed() { // @fixed
  final Getter g = Box(cleanCmd(src('dyn-iface-val-recv-fixed')));
  sinkCmd(g.get()); // @sink(none)
}

void decoy() { // @decoy
  final Getter a = Box(src('dyn-iface-val-recv-decoy')); // @source(dyn-iface-val-recv-decoy)
  final Getter b = Box('constant');
  sinkCmd(b.get()); // @sink(none) @nosink(dyn-iface-val-recv-decoy)
  sinkSql(a.get()); // @sink(dyn-iface-val-recv-decoy)
}
