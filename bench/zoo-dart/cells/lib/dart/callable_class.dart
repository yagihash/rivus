// @cell(F/dart/callable-class)
// @shape: an object of a class with a call method is called like a function (the decoy has the value in the object, and the call argument is the constant)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/callable-class/vuln', vuln);
  zooCase('F/dart/callable-class/fixed', fixed);
  zooCase('F/dart/callable-class/decoy', decoy);
}

class Runner {
  void call(String s) {
    sinkCmd(s); // @sink(dart-callable-class-vuln)
  }
}

void vuln() { // @vuln
  final r = Runner();
  r(src('dart-callable-class-vuln')); // @source(dart-callable-class-vuln) @miss(unmodeled_library_call,unresolved_callee) @ref(ZOO-D1)
}

class RunnerFixed {
  void call(String s) {
    sinkCmd(s); // @sink(none)
  }
}

void fixed() { // @fixed
  final r = RunnerFixed();
  r(cleanCmd(src('dart-callable-class-fixed')));
}

class RunnerDecoy {
  RunnerDecoy(this.prefix);
  final String prefix;
  void call(String s) {
    sinkCmd(s); // @sink(none) @nosink(dart-callable-class-decoy)
    sinkSql(prefix); // @sink(dart-callable-class-decoy)
  }
}

void decoy() { // @decoy
  final r = RunnerDecoy(src('dart-callable-class-decoy')); // @source(dart-callable-class-decoy) @miss(unresolved_callee) @ref(ZOO-D1)
  r('constant');
}
