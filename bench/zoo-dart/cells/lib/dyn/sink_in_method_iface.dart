// @cell(F/dyn/sink-in-method-iface)
// @shape: an object is held as an interface, and the sink is in the method that the interface calls
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-in-method-iface/vuln', vuln);
  zooCase('F/dyn/sink-in-method-iface/fixed', fixed);
  zooCase('F/dyn/sink-in-method-iface/decoy', decoy);
}

abstract class Execer {
  void exec();
}

class Runner implements Execer {
  Runner(this.cmd);
  final String cmd;

  @override
  void exec() {
    sinkCmd(cmd); // @sink(dyn-sink-in-method-iface-vuln)
  }
}

void vuln() { // @vuln
  final Execer e = Runner(src('dyn-sink-in-method-iface-vuln')); // @source(dyn-sink-in-method-iface-vuln)
  e.exec();
}

class RunnerFixed implements Execer {
  RunnerFixed(this.cmd);
  final String cmd;

  @override
  void exec() {
    sinkCmd(cmd); // @sink(none) @fp @ref(XTALK)
  }
}

void fixed() { // @fixed
  final Execer e = RunnerFixed(cleanCmd(src('dyn-sink-in-method-iface-fixed')));
  e.exec();
}

class RunnerDecoy implements Execer {
  RunnerDecoy(this.a, this.b);
  final String a;
  final String b;

  @override
  void exec() {
    sinkCmd(b); // @sink(none) @nosink(dyn-sink-in-method-iface-decoy)
    sinkSql(a); // @sink(dyn-sink-in-method-iface-decoy)
  }
}

void decoy() { // @decoy
  final Execer e = RunnerDecoy(src('dyn-sink-in-method-iface-decoy'), 'constant'); // @source(dyn-sink-in-method-iface-decoy)
  e.exec();
}
