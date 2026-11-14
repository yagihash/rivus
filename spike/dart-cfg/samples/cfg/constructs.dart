// The constructs the brief names as hard (section 5.4), and a few more.
// Each function is built into a CFG by bin/cfg.dart.

Future<int> fetch(int x) async => x;
Stream<int> numbers() async* {
  yield 1;
  yield 2;
}

void sink(Object? o) {}
bool cond() => true;

// --- async / await --------------------------------------------------------

Future<void> awaitInSequence(int a) async {
  final x = await fetch(a);
  final y = await fetch(x);
  sink(y);
}

Future<void> awaitInCondition(int a) async {
  if (await fetch(a) > 0) {
    sink(1);
  } else {
    sink(2);
  }
}

Future<void> awaitInLoop(List<int> xs) async {
  for (final x in xs) {
    sink(await fetch(x));
  }
}

Future<void> awaitFor() async {
  await for (final n in numbers()) {
    if (n == 2) break;
    sink(n);
  }
}

// --- try / catch / finally ------------------------------------------------

int finallyManyExits(int a) {
  try {
    if (a == 0) return 0; // leaves through finally
    if (a == 1) throw StateError('one'); // leaves through finally
    if (a == 2) {
      for (var i = 0; i < 3; i++) {
        if (i == 1) break; // stays in the function
      }
    }
    sink(a);
  } catch (e) {
    sink(e);
    return -1; // leaves through finally
  } finally {
    sink('finally'); // reached by 4 different exits
  }
  return a;
}

Future<void> tryAroundAwait() async {
  try {
    await fetch(1);
  } on StateError catch (e) {
    sink(e);
  } catch (e, st) {
    sink(st);
    rethrow;
  } finally {
    sink('done');
  }
}

void nestedTry() {
  try {
    try {
      sink(1);
    } finally {
      sink('inner');
    }
  } catch (e) {
    sink(e);
  }
}

// --- loops, labels --------------------------------------------------------

void labeledJumps(List<List<int>> rows) {
  outer:
  for (final row in rows) {
    for (final v in row) {
      if (v < 0) continue outer;
      if (v > 100) break outer;
      sink(v);
    }
  }
  sink('after');
}

void whileAndDo() {
  var i = 0;
  while (cond()) {
    i++;
    if (i > 3) break;
  }
  do {
    i--;
  } while (i > 0);
}

// --- switch with patterns (Dart 3) ---------------------------------------

sealed class Shape {}

class Circle extends Shape {
  Circle(this.r);
  final double r;
}

class Square extends Shape {
  Square(this.s);
  final double s;
}

double area(Shape shape) {
  switch (shape) {
    case Circle(r: final r) when r > 0:
      return 3.14 * r * r;
    case Circle():
      return 0;
    case Square(s: final s):
      return s * s;
  }
}

String describe(Object? o) => switch (o) {
      int n when n > 10 => 'big',
      int() => 'int',
      [var first, ...] => 'list $first',
      _ => 'other',
    };

// --- short circuit and conditional expressions ---------------------------

void shortCircuit(bool a, bool b) {
  if (a && (b || cond())) {
    sink(1);
  }
  sink(a ? 1 : 2);
}
