import 'lib.dart';

// A method of a `dynamic` value. The extractor does not know which method it is, and writes a static call
// of `dart:core#dynamic.invoke`, which the model of dart-core.yaml covers (the receiver and the arguments
// go to the result): the code of the project's own classes that have the method is not read, and the call
// leaves no uncertainty (DART-12: `extractor/dart/lib/exprs.dart` in `_methodInvocation`).

class Loud {
  void shout(String s) {
    sink(s); // @sink(dyn-arg)
  }
}

// the method of the value takes the argument to a sink inside: the model has nothing about the body
void dynamic_arg() {
  final a = source(); // @source(dyn-arg) @miss
  dynamic l = Loud();
  l.shout(a);
}

class Mute {
  String say(String s) => 'fixed';
}

// the method drops its argument: the model says the result is made of it
void dynamic_drop() {
  final c = source(); // @source(dyn-drop)
  dynamic m = Mute();
  final r = m.say(c);
  sink(r); // @sink(none) @fp
  sink(c); // @sink(dyn-drop)
}

class Maker {
  String make() => source(); // @source(dyn-body) @miss
}

// the method takes a source in its own body
void dynamic_body() {
  dynamic m = Maker();
  final r = m.make();
  sink(r); // @sink(dyn-body)
}
