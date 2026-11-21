// @cell(F/call/multiret)
// @shape: a function returns two values (a record in Dart), and the caller uses one of them
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/call/multiret/vuln', vuln);
  zooCase('F/call/multiret/fixed', fixed);
  zooCase('F/call/multiret/decoy', decoy);
}

(String, String) pair(String a, String b) => (a, b);

void vuln() { // @vuln
  final (x, _) = pair(src('call-multiret-vuln'), 'constant'); // @source(call-multiret-vuln)
  sinkCmd(x); // @sink(call-multiret-vuln)
}

void fixed() { // @fixed
  final (x, _) = pair(cleanCmd(src('call-multiret-fixed')), 'constant');
  sinkCmd(x); // @sink(none)
}

void decoy() { // @decoy
  final t = src('call-multiret-decoy'); // @source(call-multiret-decoy)
  final (x, y) = pair('constant', t);
  sinkCmd(x); // @sink(none) @nosink(call-multiret-decoy)
  sinkSql(y); // @sink(call-multiret-decoy)
}
