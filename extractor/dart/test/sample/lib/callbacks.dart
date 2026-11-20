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

// A rule names the function a call of a function value reaches: the one a
// variable holds, or the one the caller passes (yagihash/rivus #54).

void sinkViaVariable() {
  final emit = sink;
  final x = source(); // @source(cb-sinkvar)
  emit(x); // @sink(cb-sinkvar)
}

void run(void Function(String) f, String s) {
  f(s); // @sink(cb-sinkparam)
}

void quiet(String s) {}

void sinkViaParameter() {
  final x = source(); // @source(cb-sinkparam)
  run(sink, x);
}

// no flow: this caller passes a function that is not a sink
void quietViaParameter() {
  final y = source();
  run(quiet, y);
}
