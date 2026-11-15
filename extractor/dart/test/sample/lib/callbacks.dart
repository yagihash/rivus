import 'lib.dart';

// A function handed to a function-typed parameter and called there: the
// callee is found from what the callers pass.

String apply(String Function(String) f, String s) => f(s);

String ident(String s) => s;

void viaTearOff() {
  final x = source(); // @source(cb-tearoff)
  sink(apply(ident, x)); // @sink(cb-tearoff)
}

void viaLiteral() {
  final x = source(); // @source(cb-literal)
  sink(apply((s) => s, x)); // @sink(cb-literal)
}

Stream<String> produce() async* {
  yield source(); // @source(yield)
}

Future<void> consume() async {
  await for (final x in produce()) {
    sink(x); // @sink(yield)
  }
}

Iterable<String> numbers() sync* {
  yield source(); // @source(syncyield)
}

void readNumbers() {
  for (final n in numbers()) {
    sink(n); // @sink(syncyield)
  }
}

void viaCapturedCallback() {
  final t = source(); // @source(cb-capture)
  String Function() get = () => t;
  sink(get()); // @sink(cb-capture)
}
