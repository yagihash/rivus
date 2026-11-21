// @cell(F/dyn/method-value-iface)
// @shape: a tear-off of an interface method (g = i.get) whose implementation holds the value; the decoy binds two objects
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/method-value-iface/vuln', vuln);
  zooCase('F/dyn/method-value-iface/fixed', fixed);
  zooCase('F/dyn/method-value-iface/decoy', decoy);
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

void vuln() { // @vuln
  final Getter i = Box(src('dyn-method-value-iface-vuln')); // @source(dyn-method-value-iface-vuln) @miss(unmodeled_library_call) @ref(R-14,PARITY)
  final g = i.get;
  sinkCmd(g()); // @sink(dyn-method-value-iface-vuln)
}

void fixed() { // @fixed
  final Getter i = Box(cleanCmd(src('dyn-method-value-iface-fixed')));
  final g = i.get;
  sinkCmd(g()); // @sink(none)
}

void decoy() { // @decoy
  final Getter a = Box(src('dyn-method-value-iface-decoy')); // @source(dyn-method-value-iface-decoy) @miss(unmodeled_library_call) @ref(R-14,PARITY)
  final Getter b = Box('constant');
  final ga = a.get;
  final gb = b.get;
  sinkCmd(gb()); // @sink(none) @nosink(dyn-method-value-iface-decoy)
  sinkSql(ga()); // @sink(dyn-method-value-iface-decoy)
}
