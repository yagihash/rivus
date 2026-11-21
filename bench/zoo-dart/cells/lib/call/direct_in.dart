// @cell(F/call/direct-in)
// @shape: the source is the type of a parameter of the handler (In), and the handler passes its text to a sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/call/direct-in/vuln', vuln);
  zooCase('F/call/direct-in/fixed', fixed);
  zooCase('F/call/direct-in/decoy', decoy);
}

void handle(In req) { // @source(call-direct-in-vuln)
  sinkCmd(req.value); // @sink(call-direct-in-vuln)
}

void vuln() { // @vuln
  handle(In('call-direct-in-vuln'));
}

void handleFixed(In req) {
  sinkCmd(cleanCmd(req.value)); // @sink(none)
}

void fixed() { // @fixed
  handleFixed(In('call-direct-in-fixed'));
}

void handleDecoy(In req, String other) { // @source(call-direct-in-decoy)
  sinkCmd(other); // @sink(none) @nosink(call-direct-in-decoy)
  sinkSql(req.value); // @sink(call-direct-in-decoy)
}

void decoy() { // @decoy
  handleDecoy(In('call-direct-in-decoy'), 'constant');
}
