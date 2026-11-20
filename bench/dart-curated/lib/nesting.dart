import 'helpers.dart';
import 'lib.dart';

// Closures inside closures, callbacks that are called twice, callbacks that
// take callbacks, generic helpers.

void nested_a() {
  final a = source(); // @source(nest-a) @miss
  final r = transform('const', (x) {
    return transform(a, (y) => y);
  });
  sink(r); // @sink(nest-a) @nosink(nest-b)
}

void nested_b() {
  final b = source(); // @source(nest-b) @miss
  final r = transform('const', (x) {
    return transform(b, (y) => y);
  });
  sink(r); // @sink(nest-b) @nosink(nest-a)
}

// the inner closure uses the parameter of the outer one
void nested_param() {
  final a = source(); // @source(nest-param)
  final r = transform(a, (x) => transform(x, (y) => y));
  sink(r); // @sink(nest-param)
}

// the callback is called twice: the second call gets the result of the first
void twice_a() {
  final a = source(); // @source(twice-a)
  sink(twice(a, (x) => x)); // @sink(twice-a) @nosink(twice-b)
}

void twice_b() {
  final b = source(); // @source(twice-b)
  sink(twice(b, (x) => x)); // @sink(twice-b) @nosink(twice-a)
}

// the callback is called twice with different arguments, one clean
void twice_args() {
  final a = source(); // @source(twice-args) @miss
  final out = <String>[];
  fireAndForget(a, (x) => out.add(x));
  fireAndForget('const', (x) => out.add(x));
  sink(out.first); // @sink(twice-args)
}

// the callback gets another callback
void takes_callback_a() {
  final a = source(); // @source(takes-a) @miss
  final r = viaInner('const', (g) => g(a));
  sink(r); // @sink(takes-a) @nosink(takes-b)
}

void takes_callback_b() {
  final b = source(); // @source(takes-b) @miss
  final r = viaInner('const', (g) => g(b));
  sink(r); // @sink(takes-b) @nosink(takes-a)
}

// the generic helper, two instantiations
void generic_a() {
  final a = source(); // @source(gen-a)
  sink(applyTo<String>(a, (x) => x)); // @sink(gen-a) @nosink(gen-b)
}

void generic_b() {
  final b = source(); // @source(gen-b)
  sink(applyTo(b, (x) => x)); // @sink(gen-b) @nosink(gen-a)
}

void generic_int() {
  final t = source(); // @source(gen-int)
  final n = applyTo<List<String>>([t], (x) => x);
  sink(n.first); // @sink(gen-int)
}

// a function-typed local, called in a loop
void callback_in_loop() {
  final a = source(); // @source(loop)
  String Function(String) f = (x) => x;
  var v = 'const';
  for (var i = 0; i < 3; i++) {
    v = f(a);
  }
  sink(v); // @sink(loop)
}

// the callback is chosen by a condition
void choose(bool flag) {
  final a = source(); // @source(choose)
  final f = flag ? (String x) => x : (String x) => 'fixed';
  sink(f(a)); // @sink(choose)
}

// `Function` without a signature, and a typedef
void any_function() {
  final a = source(); // @source(any-fn)
  sink(callAny((String x) => x, a)); // @sink(any-fn)
}

void typedef_mapper() {
  final a = source(); // @source(typedef)
  sink(mapWith((x) => x, a)); // @sink(typedef)
}

// the closure is stored in a variable first, and given to two helpers
void shared_closure() {
  final a = source(); // @source(shared)
  String id(String x) => x;
  sink(transform(a, id)); // @sink(shared)
  sink(mapWith(id, 'const')); // @sink(none)
}

// an immediately invoked closure
void iife() {
  final a = source(); // @source(iife)
  final r = (() => a)();
  sink(r); // @sink(iife)
}

// a closure that calls itself through a local function (recursion)
void local_recursion() {
  final a = source(); // @source(local-rec)
  String go(String x, int n) => n == 0 ? x : go(x, n - 1);
  sink(go(a, 3)); // @sink(local-rec)
}

// currying: a closure that returns a closure
void curried() {
  final a = source(); // @source(curry) @miss
  String Function(String) Function(String) add = (p) => (q) => p + q;
  sink(add(a)('x')); // @sink(curry)
}

// composition of two functions
String Function(String) compose(String Function(String) f, String Function(String) g) => (x) => g(f(x));

void composed_a() {
  final a = source(); // @source(compose-a)
  sink(compose((x) => x, (y) => y)(a)); // @sink(compose-a) @nosink(compose-b)
}

void composed_b() {
  final b = source(); // @source(compose-b)
  sink(compose((x) => x, (y) => y)(b)); // @sink(compose-b) @nosink(compose-a)
}
