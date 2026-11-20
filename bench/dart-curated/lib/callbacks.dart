import 'dart:async';

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
  final t = source(); // @source(btn-a) @miss(silent)
  Button(onPressed: () => sink(t)); // @sink(btn-a) @nosink(btn-b)
}

void button_b() {
  final t = source(); // @source(btn-b) @miss(silent)
  Button(onPressed: () => sink(t)); // @sink(btn-b) @nosink(btn-a)
}

void timer_run() {
  final t = source(); // @source(timer-run) @miss(unmodeled_library_call)
  Timer.run(() => sink(t)); // @sink(timer-run)
}

void microtask() {
  final t = source(); // @source(microtask) @miss(silent)
  scheduleMicrotask(() => sink(t)); // @sink(microtask)
}

void future_delayed() {
  final t = source(); // @source(delayed) @miss(silent)
  Future.delayed(const Duration(seconds: 1), () => sink(t)); // @sink(delayed)
}

void timer_periodic() {
  final t = source(); // @source(periodic) @miss(unmodeled_library_call)
  Timer.periodic(const Duration(seconds: 1), (timer) => sink(t)); // @sink(periodic)
}

// a callback that writes a captured variable, called by a timer
void timer_write() {
  String out = '';
  final t = source(); // @source(timer-write) @miss(unmodeled_library_call)
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
