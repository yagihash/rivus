import 'dart:async';

import 'helpers.dart';
import 'lib.dart';

// A closure that captures a value and is given to a function that calls it later: the widget
// callback (`Button(onPressed: ...)`), the timers and the microtasks of dart:async. Where it is called
// is not in the function that made it; the value is in the function that made it (C-7,
// docs/DESIGN-capture.md). The flows that are found today are the ones that do not need that.

// a widget: the callback is kept in a field, and a library calls it on a tap
class Button {
  final void Function() onPressed;
  const Button({required this.onPressed});
}

void button_a() {
  final t = source(); // @source(btn-a)
  Button(onPressed: () => sink(t)); // @sink(btn-a) @nosink(btn-b)
}

void button_b() {
  final t = source(); // @source(btn-b)
  Button(onPressed: () => sink(t)); // @sink(btn-b) @nosink(btn-a)
}

void timer_run() {
  final t = source(); // @source(timer-run)
  Timer.run(() => sink(t)); // @sink(timer-run)
}

void microtask() {
  final t = source(); // @source(microtask)
  scheduleMicrotask(() => sink(t)); // @sink(microtask)
}

void future_delayed() {
  final t = source(); // @source(delayed)
  Future.delayed(const Duration(seconds: 1), () => sink(t)); // @sink(delayed)
}

void timer_periodic() {
  final t = source(); // @source(periodic)
  Timer.periodic(const Duration(seconds: 1), (timer) => sink(t)); // @sink(periodic)
}

// a callback that writes a captured variable, called by a timer
void timer_write() {
  String out = '';
  final t = source(); // @source(timer-write)
  Timer.run(() {
    out = t;
  });
  sink(out); // @sink(timer-write)
}

// the closure uses its own parameter (what the timer gives), not the captured value
void timer_param() {
  final t = source(); // @source(timer-param)
  Timer.periodic(const Duration(seconds: 1), (timer) => sink(timer)); // @sink(none)
  sink(t); // @sink(timer-param)
}

// the closure is made and never called: a rule that takes it to run where it is made reports it
void never_called() {
  final t = source(); // @source(never)
  final f = () => sink(t); // @sink(none)
  sink(t); // @sink(never)
}

// the variable is captured by reference: it is tainted after the closure is made, before the helper calls it
void late_taint() {
  String? x;
  final f = () => sink(x); // @sink(late)
  x = source(); // @source(late)
  run(f);
}

// the closure is called where it is made, and also given to a helper: one finding
void seen_and_passed() {
  final a = source(); // @source(seen)
  final f = () => sink(a); // @sink(seen)
  f();
  run(f);
}

// a captured function that the closure calls with a value of the caller: capture.dart has `relay-c-a` and
// `relay-c-b` (the capture of a closure that is called by a helper, PARAM-CALL); the same in the callback
void captured_fn() {
  final a = source(); // @source(cap-fn) @miss(assumed_call)
  void g(String s) => sink(s); // @sink(cap-fn)
  run(() => g(a));
}

// what the closure returns is what it captured (`items.map((x) => x + a)`, `Future.then((_) => a)`): the result of a
// closure has no place to go where it is made. After C-7: `@miss(assumed_call)` (the result is not applied).
void cap_result() {
  final a = source(); // @source(cap-result)
  final x = [1].map((e) => a).first;
  sink(x); // @sink(cap-result)
}

void cap_result_then() {
  final a = source(); // @source(cap-result-then)
  Future.value(1).then((_) => a).then((x) => sink(x)); // @sink(cap-result-then)
}

// the closure is only called where it is made (by name, and as a literal): the call has what it
// captured, and nothing is applied at the place it is made (C-7): no assumed_call, one finding each
void inline_only() {
  final a = source(); // @source(inline-only)
  final f = () => sink(a); // @sink(inline-only)
  f();
}

void inline_literal() {
  final a = source(); // @source(inline-literal)
  (() => sink(a))(); // @sink(inline-literal)
}
