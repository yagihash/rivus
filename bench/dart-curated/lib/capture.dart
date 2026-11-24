import 'helpers.dart';
import 'lib.dart';

// A closure that writes a variable it captured, and what comes out of it.

void write_a() {
  String? out;
  final t = source(); // @source(write-a)
  run(() {
    out = t;
  });
  sink(out); // @sink(write-a) @nosink(write-b)
}

void write_b() {
  String? out;
  final t = source(); // @source(write-b)
  run(() {
    out = t;
  });
  sink(out); // @sink(write-b) @nosink(write-a)
}

// the closure writes a constant: nothing comes out
void write_const() {
  String out = '';
  final t = source(); // @source(write-const)
  run(() {
    out = 'fixed';
  });
  sink(out); // @sink(none)
  sink(t); // @sink(write-const)
}

// the callback writes what the helper gave it into the captured variable
void write_param() {
  String out = '';
  final t = source(); // @source(write-param) @miss(depth_limit)
  fireAndForget(t, (x) {
    out = x;
  });
  sink(out); // @sink(write-param)
}

// a list that the closure adds to
void add_to_list() {
  final items = <String>[];
  final t = source(); // @source(add-list)
  run(() {
    items.add(t);
  });
  sink(items.first); // @sink(add-list)
}

// a relay through the parameter: the value goes into a helper that calls a
// second helper, which calls the closure
String relayParam(String s, String Function(String) f) => transform(s, f);

void relay_param_a() {
  final a = source(); // @source(relay-p-a)
  sink(relayParam(a, (x) => x)); // @sink(relay-p-a) @nosink(relay-p-b)
}

void relay_param_b() {
  final b = source(); // @source(relay-p-b)
  sink(relayParam(b, (x) => x)); // @sink(relay-p-b) @nosink(relay-p-a)
}

// a relay through the capture: the closure that is given to the second helper
// captures the callback of the first
void relayCapture(void Function() f) {
  run(() => f());
}

void relay_cap_a() {
  final a = source(); // @source(relay-c-a)
  relayCapture(() => sink(a)); // @sink(relay-c-a) @nosink(relay-c-b)
}

void relay_cap_b() {
  final b = source(); // @source(relay-c-b)
  relayCapture(() => sink(b)); // @sink(relay-c-b) @nosink(relay-c-a)
}

// a relay of both kinds: the parameter, then the capture
String relayBoth(String s, String Function(String) f) {
  return transform(s, (x) => f(x));
}

void relay_both_a() {
  final a = source(); // @source(relay-b-a)
  sink(relayBoth(a, (x) => x)); // @sink(relay-b-a) @nosink(relay-b-b)
}

void relay_both_b() {
  final b = source(); // @source(relay-b-b)
  sink(relayBoth(b, (x) => x)); // @sink(relay-b-b) @nosink(relay-b-a)
}

// a closure returned by a function holds a captured value
void holder_a() {
  final a = source(); // @source(holder-a) @miss(silent)
  final get = holder(a);
  sink(get()); // @sink(holder-a) @nosink(holder-b)
}

void holder_b() {
  final b = source(); // @source(holder-b) @miss(silent)
  final get = holder(b);
  sink(get()); // @sink(holder-b) @nosink(holder-a)
}
