// @cell(F/dart/named-args)
// @shape: a value is given as a named argument, in an order other than the parameters' (the decoy gives the value to the other name)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/named-args/vuln', vuln);
  zooCase('F/dart/named-args/fixed', fixed);
  zooCase('F/dart/named-args/decoy', decoy);
}

void run({required String shell, required String cmd}) {
  sinkCmd(cmd); // @sink(dart-named-args-vuln)
  sinkPath('/srv/base', shell); // @sink(none)
}

void vuln() { // @vuln
  run(cmd: src('dart-named-args-vuln'), shell: 'sh'); // @source(dart-named-args-vuln)
}

void runFixed({required String shell, required String cmd}) {
  sinkCmd(cmd); // @sink(none)
  sinkPath('/srv/base', shell); // @sink(none)
}

void fixed() { // @fixed
  runFixed(cmd: cleanCmd(src('dart-named-args-fixed')), shell: 'sh');
}

void runDecoy({required String shell, required String cmd}) {
  sinkCmd(cmd); // @sink(none) @nosink(dart-named-args-decoy)
  sinkSql(shell); // @sink(dart-named-args-decoy)
}

void decoy() { // @decoy
  runDecoy(cmd: 'constant', shell: src('dart-named-args-decoy')); // @source(dart-named-args-decoy)
}
