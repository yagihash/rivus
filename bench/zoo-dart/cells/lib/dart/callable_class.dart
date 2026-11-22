// @cell(F/dart/callable-class)
// @shape: an instance of a class that has a `call` method is called like a function: r(x) is r.call(x)
// @na(go: a value of a Go type has no `call` method; a func value is the nearest, and F/clos has it)
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
  r(src('dart-callable-class-vuln')); // @source(dart-callable-class-vuln)
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
  void call(String a, String b) {
    sinkCmd(b); // @sink(none) @nosink(dart-callable-class-decoy)
    sinkSql(a); // @sink(dart-callable-class-decoy)
  }
}

void decoy() { // @decoy
  final r = RunnerDecoy();
  r(src('dart-callable-class-decoy'), 'constant'); // @source(dart-callable-class-decoy)
}
