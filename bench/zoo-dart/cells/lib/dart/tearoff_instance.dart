// @cell(F/dart/tearoff-instance)
// @na(go: the method value is dyn/method-value; this cell adds Stream.listen and onPressed, which are the common Flutter forms)
// @shape: the tear-off of an instance method (the receiver is bound, the argument comes later) is called, also as the callback of Stream.listen and of a widget's onPressed (the decoy has the value in the receiver and the constant as the argument)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/tearoff-instance/vuln', vuln);
  zooCase('F/dart/tearoff-instance/vuln2', vuln2);
  zooCase('F/dart/tearoff-instance/vuln3', vuln3);
  zooCase('F/dart/tearoff-instance/fixed', fixed);
  zooCase('F/dart/tearoff-instance/decoy', decoy);
}

class Printer {
  Printer(this.prefix);
  final String prefix;
  void run(String arg) {
    sinkCmd(arg); // @sink(dart-tearoff-instance-vuln)
  }
}

void vuln() { // @vuln
  final f = Printer('constant').run;
  f(src('dart-tearoff-instance-vuln')); // @source(dart-tearoff-instance-vuln)
}

// the callback of Stream.listen, which is an instance method of a controller
class Controller {
  Controller(this.name);
  final String name;
  void onData(String event) {
    sinkCmd(event); // @sink(dart-tearoff-instance-vuln2)
  }
}

Future<void> vuln2() async { // @vuln
  final c = Controller('list');
  final stream = Stream.value(src('dart-tearoff-instance-vuln2')); // @source(dart-tearoff-instance-vuln2)
  await stream.listen(c.onData).asFuture<void>();
}

// the onPressed of a widget (a skeleton of Flutter's: the framework is not in the zoo)
class Button {
  Button({required this.onPressed});
  final void Function(String) onPressed;
  void press(String text) => onPressed(text);
}

class Form {
  void submit(String text) {
    sinkCmd(text); // @sink(dart-tearoff-instance-vuln3)
  }

  Button build() => Button(onPressed: submit);
}

void vuln3() { // @vuln
  final b = Form().build();
  b.press(src('dart-tearoff-instance-vuln3')); // @source(dart-tearoff-instance-vuln3) @miss(unmodeled_library_call) @ref(R-14)
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
  f(cleanCmd(src('dart-tearoff-instance-fixed')));
}

class PrinterDecoy {
  PrinterDecoy(this.prefix);
  final String prefix;
  void run(String arg) {
    sinkCmd(arg); // @sink(none) @nosink(dart-tearoff-instance-decoy)
    sinkSql(prefix); // @sink(dart-tearoff-instance-decoy)
  }
}

void decoy() { // @decoy
  final f = PrinterDecoy(src('dart-tearoff-instance-decoy')).run; // @source(dart-tearoff-instance-decoy)
  f('constant');
}
