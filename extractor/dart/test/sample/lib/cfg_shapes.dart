// The shapes of control flow that the Dart spike's tests (spike/dart-cfg, removed in #38) checked, kept
// for the extractor itself: the finally block once for every way out of the try, and the operators that
// are control flow. cmd/rivus/dart_test.go (TestDartExtractorControlFlowShapes) reads the IR of these.
// No source or sink is here: the flows of these forms are in review_try.dart and syntax.dart.

void fin() {}
void work(Object? o) {}
bool cond() => true;

// the finally is reached by four ways out: the end of the try, a return in it, a throw in it, and the catch
int finallyManyExits(int a) {
  try {
    if (a == 0) return 0;
    if (a == 1) throw StateError('one');
    work(a);
  } catch (e) {
    work(e);
    return -1;
  } finally {
    fin();
  }
  return a;
}

// the inner finally runs on the way out of the inner try, and on the way out by an exception
void nestedTry() {
  try {
    try {
      work(1);
    } finally {
      fin();
    }
  } catch (e) {
    work(e);
  }
}

// && and || and ?: are control flow: the right side is a block that is entered by a branch
void shortCircuit(bool a, bool b) {
  if (a && (b || cond())) {
    work(1);
  }
  work(a ? 1 : 2);
}
