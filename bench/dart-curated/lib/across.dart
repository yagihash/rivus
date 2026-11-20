import 'package:shared/shared.dart';

import 'lib.dart';

// The helper is in another package.

void across_a() {
  final a = source(); // @source(across-a)
  sink(withIn(a, (x) => x)); // @sink(across-a) @nosink(across-b)
}

void across_b() {
  final b = source(); // @source(across-b)
  sink(withIn(b, (x) => x)); // @sink(across-b) @nosink(across-a)
}

void across_each() {
  final items = [source()]; // @source(across-each)
  each(items, (x) => sink(x)); // @sink(across-each)
}

void across_hub() {
  final h = Hub();
  h.on((s) => sink(s)); // @sink(across-hub)
  h.emit(source()); // @source(across-hub) @miss(silent)
}

void across_capture() {
  String? out;
  final t = source(); // @source(across-cap) @miss(silent)
  each([1], (i) {
    out = t;
  });
  sink(out); // @sink(across-cap)
}
