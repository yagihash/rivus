// @cell(F/dart/patterns-record)
// @shape: a variable that a pattern declares (a record pattern, an if-case, a switch case; also one that a closure assigns) gets the value (the decoy declares two and sinks the other)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/patterns-record/vuln', vuln);
  zooCase('F/dart/patterns-record/vuln2', vuln2);
  zooCase('F/dart/patterns-record/vuln3', vuln3);
  zooCase('F/dart/patterns-record/vuln4', vuln4);
  zooCase('F/dart/patterns-record/fixed', fixed);
  zooCase('F/dart/patterns-record/decoy', decoy);
}

void vuln() { // @vuln
  final record = (src('dart-patterns-record-vuln'), 'constant'); // @source(dart-patterns-record-vuln)
  final (a, _) = record;
  sinkCmd(a); // @sink(dart-patterns-record-vuln)
}

void vuln2() { // @vuln
  final Object r = (src('dart-patterns-record-vuln2'), 1); // @source(dart-patterns-record-vuln2)
  if (r case (String s, int _)) {
    sinkCmd(s); // @sink(dart-patterns-record-vuln2)
  }
}

void vuln3() { // @vuln
  final Object r = [src('dart-patterns-record-vuln3'), 'constant']; // @source(dart-patterns-record-vuln3)
  switch (r) {
    case [String s, String _]:
      sinkCmd(s); // @sink(dart-patterns-record-vuln3)
  }
}

// the variable that a pattern declares is captured by a closure and assigned there
void vuln4() { // @vuln
  var (a, _) = ('constant', 1);
  void load() {
    a = src('dart-patterns-record-vuln4'); // @source(dart-patterns-record-vuln4) @miss(silent) @ref(R-15)
  }

  load();
  sinkCmd(a); // @sink(dart-patterns-record-vuln4)
}

void fixed() { // @fixed
  final record = (cleanCmd(src('dart-patterns-record-fixed')), 'constant');
  final (a, _) = record;
  sinkCmd(a); // @sink(none)
}

void decoy() { // @decoy
  final record = (src('dart-patterns-record-decoy'), 'constant'); // @source(dart-patterns-record-decoy)
  final (a, b) = record;
  sinkCmd(b); // @sink(none) @nosink(dart-patterns-record-decoy)
  sinkSql(a); // @sink(dart-patterns-record-decoy)
}
