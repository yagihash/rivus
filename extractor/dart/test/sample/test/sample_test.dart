import 'package:matcher/matcher.dart';
import 'package:sample/lib.dart';

// A test of the package: the build does not ship it, so it is not read unless --include-tests asks (ADR-062).
void main() {
  final s = source();
  sink(s);
  isEmpty.matches(s, {});
}
