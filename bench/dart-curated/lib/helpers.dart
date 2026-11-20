// Helpers that take a function, shared by the callers of the other files.

// the callback gets the value, and what it returns is returned
String transform(String s, String Function(String) f) => f(s);

// the callback is called with a constant: what it holds is the caller's
void produce(void Function(String) f) => f('const');

// the callback is called for the side effect
void run(void Function() f) => f();

// the callback is called twice
String twice(String s, String Function(String) f) => f(f(s));

// the function is a typedef
typedef Mapper = String Function(String);
String mapWith(Mapper m, String s) => m(s);

// the helper is generic
T applyTo<T>(T value, T Function(T) f) => f(value);

// the callback gets another callback
String viaInner(String s, String Function(String Function(String)) outer) => outer((x) => x + s);

// with the `Function` type (no signature)
dynamic callAny(Function f, String s) => f(s);

// a helper that calls the callback and drops the result
void fireAndForget(String s, void Function(String) f) {
  f(s);
}

// a helper that returns a closure that holds its argument
String Function() holder(String s) => () => s;

// the callback is named in a field of the helper
class Runner {
  final String Function(String) f;
  Runner(this.f);
  String go(String s) => f(s);
}
