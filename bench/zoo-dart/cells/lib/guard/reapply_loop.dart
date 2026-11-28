// @cell(F/guard/reapply-loop)
// @shape: a call is applied again in a loop, and the return after the loop depends on a parameter (the case runs both values)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/guard/reapply-loop/vuln', vuln);
  zooCase('F/guard/reapply-loop/fixed', fixed);
  zooCase('F/guard/reapply-loop/decoy', decoy);
}

String pass(bool flag, String s) {
  if (flag) {
    return s;
  }
  return '';
}

// the call is applied again with what it gave back (r + s), and skip decides what is returned
String loop(bool skip, String s) {
  var r = '';
  for (var i = 0; i < 3; i++) {
    r = pass(true, r + s);
  }
  if (skip) {
    return '';
  }
  return r;
}

void viaSink(bool skip, String s) {
  sinkCmd(loop(skip, s)); // @sink(guard-reapply-loop-vuln)
}

void vuln() { // @vuln
  final v = src('guard-reapply-loop-vuln'); // @source(guard-reapply-loop-vuln)
  viaSink(false, v);
  viaSink(true, v);
  pass(false, v); // the way of pass that gives back ''
}

void viaSinkFixed(bool skip, String s) {
  sinkCmd(cleanCmd(loop(skip, s))); // @sink(none)
}

void fixed() { // @fixed
  final v = src('guard-reapply-loop-fixed');
  viaSinkFixed(false, v);
  viaSinkFixed(true, v);
}

// skip is true on the line of the command: the function gives back '' there, and the value reaches the other line only
void decoy() { // @decoy
  final v = src('guard-reapply-loop-decoy'); // @source(guard-reapply-loop-decoy)
  sinkCmd(loop(true, v)); // @sink(none) @nosink(guard-reapply-loop-decoy)
  sinkSql(loop(false, v)); // @sink(guard-reapply-loop-decoy)
}
