import 'lib.dart';

// Flows through the forms of control flow that the spike's tests had (yagihash/rivus #58): a finally
// that a return, a break and a continue go through, the labels, `??`, and a switch whose cases are not empty.

String finallyThenReturn() {
  var x = 'safe';
  try {
    x = source(); // @source(is-58-return)
    return 'done';
  } finally {
    sink(x); // @sink(is-58-return)
  }
}

void finallyThenBreak() {
  var x = 'safe';
  for (var i = 0; i < 2; i++) {
    try {
      x = source(); // @source(is-58-break)
      break;
    } finally {
      sink(x); // @sink(is-58-break)
    }
  }
}

void finallyThenContinue(List<int> xs) {
  var x = 'safe';
  for (final _ in xs) {
    try {
      x = source(); // @source(is-58-continue)
      continue;
    } finally {
      sink(x); // @sink(is-58-continue)
    }
  }
}

void labeledBreakKeepsTheValue(List<List<int>> rows) {
  var last = 'safe';
  outer:
  for (final row in rows) {
    for (final v in row) {
      if (v > 100) {
        last = source(); // @source(is-58-label)
        break outer;
      }
    }
  }
  sink(last); // @sink(is-58-label)
}

void labeledContinueSkipsTheRest(List<List<int>> rows) {
  var last = 'safe';
  outer:
  for (final row in rows) {
    for (final v in row) {
      if (v < 0) continue outer;
      last = 'fixed';
    }
    last = source(); // @source(is-58-continue-label)
  }
  sink(last); // @sink(is-58-continue-label)
}

String ifNull(String? a) {
  final v = a ?? source(); // @source(is-58-ifnull)
  sink(v); // @sink(is-58-ifnull)
  return v;
}

void sanitizedByIfNull(String? a) {
  final v = sanitize(a ?? source());
  sink(v); // @sink(none)
}

void switchOfNonEmptyCases(int k) {
  var q = 'safe';
  switch (k) {
    case 1:
      q = source(); // @source(is-58-switch)
      sink(q); // @sink(is-58-switch)
    case 2:
      q = 'fixed';
      sink(q); // @sink(none)
    default:
      sink(q); // @sink(none)
  }
}
