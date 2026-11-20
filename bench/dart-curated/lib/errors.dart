import 'helpers.dart';
import 'lib.dart';

// Errors and how a value goes through them.

void catch_thrown() {
  try {
    throw source(); // @source(thrown)
  } catch (e) {
    sink(e); // @sink(thrown)
  }
}

void catch_other_clean() {
  try {
    final a = source(); // @source(catch-clean)
    sink(a); // @sink(catch-clean)
  } catch (e) {
    sink('const'); // @sink(none)
  }
}

void finally_value() {
  String out = '';
  final a = source(); // @source(finally)
  try {
    out = 'x';
  } finally {
    out = a;
  }
  sink(out); // @sink(finally)
}

class AppError implements Exception {
  final String detail;
  AppError(this.detail);
}

void catch_typed() {
  try {
    throw AppError(source()); // @source(typed)
  } on AppError catch (e) {
    sink(e.detail); // @sink(typed)
  }
}

String mayThrow(String s) {
  if (s.isEmpty) throw AppError(s);
  return s;
}

void throw_through_helper() {
  final a = source(); // @source(through) @miss(silent)
  try {
    transform(a, (x) => throw AppError(x));
  } on AppError catch (e) {
    sink(e.detail); // @sink(through)
  }
}

void throw_from_callee() {
  final a = source(); // @source(callee-throw)
  try {
    mayThrow(a);
  } catch (e) {
    sink((e as AppError).detail); // @sink(callee-throw)
  }
}

void rethrow_flow() {
  final a = source(); // @source(rethrow)
  try {
    try {
      throw AppError(a);
    } catch (e) {
      rethrow;
    }
  } on AppError catch (e) {
    sink(e.detail); // @sink(rethrow)
  }
}

// the error of a future is given to the handler
Future<void> future_error() async {
  try {
    await Future<void>.error(AppError(source())); // @source(future-error) @miss(silent)
  } on AppError catch (e) {
    sink(e.detail); // @sink(future-error)
  }
}

// the value is given to the callback of onError
void on_error() {
  final a = source(); // @source(on-error) @miss(silent)
  Future<String>.error(a).then((v) => v, onError: (e) => sink(e)); // @sink(on-error)
}

// the value from a try that is returned
String tryReturn() {
  try {
    return source(); // @source(try-return)
  } catch (_) {
    return '';
  }
}

void try_return_use() {
  sink(tryReturn()); // @sink(try-return)
}

// a callback that may throw, and its result is not used
void guarded() {
  final a = source(); // @source(guarded)
  var out = 'const';
  try {
    out = transform(a, (x) => x);
  } catch (_) {}
  sink(out); // @sink(guarded)
}
