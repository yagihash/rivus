// @cell(F/call/direct)
// @shape: a value of a source goes to a sink in the same function
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/call/direct/vuln', vuln);
  zooCase('F/call/direct/fixed', fixed);
  zooCase('F/call/direct/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('call-direct-vuln'); // @source(call-direct-vuln)
  sinkCmd(v); // @sink(call-direct-vuln)
}

void fixed() { // @fixed
  final v = cleanCmd(src('call-direct-fixed'));
  sinkCmd(v); // @sink(none)
}

void decoy() { // @decoy
  final t = src('call-direct-decoy'); // @source(call-direct-decoy)
  sinkCmd('constant'); // @sink(none) @nosink(call-direct-decoy)
  sinkCmd(t); // @sink(call-direct-decoy)
}
