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


// --- #70: a typed catch, and a dynamic field read, reach the fields of the object that was thrown

void catchTyped() {
  try {
    throw Boom(source()); // @source(is-70-typed)
  } on Boom catch (e) {
    sink(e.msg); // @sink(is-70-typed)
  }
}

void catchDynamic() {
  try {
    throw Boom(source()); // @source(is-70-dynamic)
  } catch (e) {
    final dynamic d = e;
    sink(d.msg); // @sink(is-70-dynamic)
  }
}

void thrower70() {
  throw Boom(source()); // @source(is-70-callee)
}

void catchTypedCallee() {
  try {
    thrower70();
  } on Boom catch (e) {
    sink(e.msg); // @sink(is-70-callee)
  }
}

void catchRethrown() {
  try {
    try {
      throw Boom(source()); // @source(is-70-rethrow)
    } on Boom {
      rethrow;
    }
  } on Boom catch (e) {
    sink(e.msg); // @sink(is-70-rethrow)
  }
}
