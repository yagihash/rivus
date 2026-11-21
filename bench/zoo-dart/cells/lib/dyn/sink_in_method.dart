// @cell(F/dyn/sink-in-method)
// @shape: a constructor stores the value in an object, and the sink is in a method of the object
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-in-method/vuln', vuln);
  zooCase('F/dyn/sink-in-method/fixed', fixed);
  zooCase('F/dyn/sink-in-method/decoy', decoy);
}

class Runner {
  Runner(this.cmd);
  final String cmd;

  void exec() {
    sinkCmd(cmd); // @sink(dyn-sink-in-method-vuln)
  }
}

void vuln() { // @vuln
  Runner(src('dyn-sink-in-method-vuln')).exec(); // @source(dyn-sink-in-method-vuln)
}

class RunnerFixed {
  RunnerFixed(this.cmd);
  final String cmd;

  void exec() {
    sinkCmd(cmd); // @sink(none)
  }
}

void fixed() { // @fixed
  RunnerFixed(cleanCmd(src('dyn-sink-in-method-fixed'))).exec();
}

class RunnerDecoy {
  RunnerDecoy(this.a, this.b);
  final String a;
  final String b;

  void exec() {
    sinkCmd(b); // @sink(none) @nosink(dyn-sink-in-method-decoy)
    sinkSql(a); // @sink(dyn-sink-in-method-decoy)
  }
}

void decoy() { // @decoy
  final r = RunnerDecoy(src('dyn-sink-in-method-decoy'), 'constant'); // @source(dyn-sink-in-method-decoy)
  r.exec();
}
