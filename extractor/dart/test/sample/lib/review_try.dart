import 'lib.dart';

// What a review of the extractor asked for (REVIEW-2026-11-19): exceptions.

void mayThrow() {}

// --- DART-1: a throw inside a catch clause still runs the finally -----------

void finallyAfterThrowInCatch() {
  var x = 'safe';
  try {
    mayThrow();
  } catch (e) {
    x = source(); // @source(rv-catch-fin)
    throw StateError('again');
  } finally {
    sink(x); // @sink(rv-catch-fin)
  }
}

void finallyAfterRethrow() {
  var x = 'safe';
  try {
    mayThrow();
  } catch (e) {
    x = source(); // @source(rv-rethrow-fin)
    rethrow;
  } finally {
    sink(x); // @sink(rv-rethrow-fin)
  }
}
