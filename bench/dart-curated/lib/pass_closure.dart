import 'helpers.dart';
import 'lib.dart';

// A closure given to a helper, by two callers: the value of one must not reach
// the sink of the other.

void ret_a() {
  final a = source(); // @source(ret-a)
  final r = transform(a, (x) => x);
  sink(r); // @sink(ret-a) @nosink(ret-b)
}

void ret_b() {
  final b = source(); // @source(ret-b)
  final r = transform(b, (x) => x);
  sink(r); // @sink(ret-b) @nosink(ret-a)
}

// the closure puts a constant of the caller's in; the helper's result is clean
void ret_clean() {
  final c = source(); // @source(ret-clean)
  final r = transform(c, (x) => 'fixed');
  sink(r); // @sink(none) @fp
  sink(c); // @sink(ret-clean)
}

// the callback is given the value: it reaches the sink inside the closure
void arg_a() {
  final a = source(); // @source(arg-a)
  fireAndForget(a, (x) => sink(x)); // @sink(arg-a) @nosink(arg-b)
}

void arg_b() {
  final b = source(); // @source(arg-b)
  fireAndForget(b, (x) => sink(x)); // @sink(arg-b) @nosink(arg-a)
}

// the helper calls the callback with a constant, the caller's source stays in the caller
void prod_a() {
  final a = source(); // @source(prod-a)
  produce((x) {
    sink(x); // @sink(none)
  });
  sink(a); // @sink(prod-a)
}

// the callback of the helper is given the value the caller captured
void run_a() {
  final a = source(); // @source(run-a) @miss
  run(() => sink(a)); // @sink(run-a) @nosink(run-b)
}

void run_b() {
  final b = source(); // @source(run-b) @miss
  run(() => sink(b)); // @sink(run-b) @nosink(run-a)
}

// a block body, and the value is changed on the way
void block_body() {
  final a = source(); // @source(block)
  final r = transform(a, (x) {
    final y = x.trim();
    return y;
  });
  sink(r); // @sink(block)
}

// a sanitizing closure: it returns a constant on one branch only
void branchy(bool ok) {
  final a = source(); // @source(branchy)
  final r = transform(a, (x) {
    if (ok) return x;
    return 'no';
  });
  sink(r); // @sink(branchy)
}
