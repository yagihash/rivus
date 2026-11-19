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
// --- CORE-2: a tainted value that is thrown reaches the catch ---------------

class Boom {
  Boom(this.msg);
  final String msg;
}

void throwAndCatchHere() {
  try {
    throw Boom(source()); // @source(rv-throw-here)
  } catch (e) {
    sink((e as Boom).msg); // @sink(rv-throw-here)
  }
}

void thrower() {
  throw Boom(source()); // @source(rv-throw-callee)
}

void catcher() {
  try {
    thrower();
  } catch (e) {
    sink((e as Boom).msg); // @sink(rv-throw-callee)
  }
}

