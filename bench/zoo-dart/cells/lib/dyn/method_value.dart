// @cell(F/dyn/method-value)
// @shape: a method tear-off (a bound method) is called; the receiver and the argument change places in the decoy
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/method-value/vuln', vuln);
  zooCase('F/dyn/method-value/fixed', fixed);
  zooCase('F/dyn/method-value/decoy', decoy);
}

class Printer {
  Printer(this.prefix);
  final String prefix;

  void run(String arg) {
    sinkCmd(arg); // @sink(dyn-method-value-vuln)
  }
}

void vuln() { // @vuln
  final f = Printer('constant').run;
  f(src('dyn-method-value-vuln')); // @source(dyn-method-value-vuln) @miss(silent) @ref(R-14,PARITY)
}

class PrinterFixed {
  PrinterFixed(this.prefix);
  final String prefix;

  void run(String arg) {
    sinkCmd(arg); // @sink(none)
  }
}

void fixed() { // @fixed
  final f = PrinterFixed('constant').run;
  f(cleanCmd(src('dyn-method-value-fixed')));
}

class PrinterDecoy {
  PrinterDecoy(this.prefix);
  final String prefix;

  void run(String arg) {
    sinkCmd(arg); // @sink(none) @fp @nosink(dyn-method-value-decoy) @fp @ref(R-14)
    sinkSql(prefix); // @sink(dyn-method-value-decoy)
  }
}

void decoy() { // @decoy
  final f = PrinterDecoy(src('dyn-method-value-decoy')).run; // @source(dyn-method-value-decoy) @miss(silent) @ref(R-14,PARITY)
  f('constant');
}
