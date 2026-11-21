// @cell(F/dart/for-in)
// @shape: the variable of a for-in loop gets the elements of a list (also a pattern variable, and one that a closure assigns) (the decoy loops over the constants and sinks the tainted element elsewhere)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/for-in/vuln', vuln);
  zooCase('F/dart/for-in/vuln2', vuln2);
  zooCase('F/dart/for-in/vuln3', vuln3);
  zooCase('F/dart/for-in/fixed', fixed);
  zooCase('F/dart/for-in/decoy', decoy);
}

void vuln() { // @vuln
  for (final x in [src('dart-for-in-vuln')]) { // @source(dart-for-in-vuln)
    sinkCmd(x); // @sink(dart-for-in-vuln)
  }
}

void vuln2() { // @vuln
  for (final (a, _) in [(src('dart-for-in-vuln2'), 1)]) { // @source(dart-for-in-vuln2)
    sinkCmd(a); // @sink(dart-for-in-vuln2)
  }
}

// the variable of the loop is captured by a closure and assigned there
void vuln3() { // @vuln
  for (var x in ['constant']) {
    void load() {
      x = src('dart-for-in-vuln3'); // @source(dart-for-in-vuln3) @miss(silent) @ref(R-15)
    }

    load();
    sinkCmd(x); // @sink(dart-for-in-vuln3)
  }
}

void fixed() { // @fixed
  for (final x in [cleanCmd(src('dart-for-in-fixed'))]) {
    sinkCmd(x); // @sink(none)
  }
}

void decoy() { // @decoy
  final t = src('dart-for-in-decoy'); // @source(dart-for-in-decoy)
  for (final x in ['constant', 'other']) {
    sinkCmd(x); // @sink(none) @nosink(dart-for-in-decoy)
  }
  for (final y in [t]) {
    sinkSql(y); // @sink(dart-for-in-decoy)
  }
}
