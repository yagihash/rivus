// @cell(F/clos/to-own-helper)
// @shape: a closure that captures a value is given to a helper of the project, which calls it
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-own-helper/vuln', vuln);
  zooCase('F/clos/to-own-helper/fixed', fixed);
  zooCase('F/clos/to-own-helper/decoy', decoy);
}

void run(void Function() f) => f();

void vuln() { // @vuln
  final v = src('clos-to-own-helper-vuln'); // @source(clos-to-own-helper-vuln)
  run(() {
    sinkCmd(v); // @sink(clos-to-own-helper-vuln)
  });
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-to-own-helper-fixed'));
  run(() {
    sinkCmd(v); // @sink(none)
  });
}

void decoy() { // @decoy
  final v = src('clos-to-own-helper-decoy'); // @source(clos-to-own-helper-decoy)
  const k = 'constant';
  run(() {
    sinkCmd(k); // @sink(none) @nosink(clos-to-own-helper-decoy)
  });
  run(() {
    sinkSql(v); // @sink(clos-to-own-helper-decoy)
  });
}
