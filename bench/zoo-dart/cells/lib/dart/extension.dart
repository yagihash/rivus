// @cell(F/dart/extension)
// @shape: an extension method gets the value as its receiver (this) and sinks it (the decoy sinks the argument, and the receiver goes to another sink)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/extension/vuln', vuln);
  zooCase('F/dart/extension/fixed', fixed);
  zooCase('F/dart/extension/decoy', decoy);
}

extension ShellVuln on String {
  void run() {
    sinkCmd(this); // @sink(dart-extension-vuln)
  }
}

void vuln() { // @vuln
  src('dart-extension-vuln').run(); // @source(dart-extension-vuln)
}

extension ShellFixed on String {
  void runFixed() {
    sinkCmd(this); // @sink(none)
  }
}

void fixed() { // @fixed
  cleanCmd(src('dart-extension-fixed')).runFixed();
}

extension ShellDecoy on String {
  void runWith(String other) {
    sinkCmd(other); // @sink(none) @nosink(dart-extension-decoy)
    sinkSql(this); // @sink(dart-extension-decoy)
  }
}

void decoy() { // @decoy
  src('dart-extension-decoy').runWith('constant'); // @source(dart-extension-decoy)
}
