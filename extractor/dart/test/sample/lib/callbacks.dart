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
