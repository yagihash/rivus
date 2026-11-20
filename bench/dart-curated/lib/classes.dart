import 'lib.dart';

// Callbacks registered in an object and called later: an emitter, a builder,
// a lazy value.

class Emitter {
  final List<void Function(String)> _subs = [];
  void on(void Function(String) f) => _subs.add(f);
  void emit(String s) {
    for (final f in _subs) {
      f(s);
    }
  }
}

void emitter_flow() {
  final e = Emitter();
  e.on((s) => sink(s)); // @sink(emit)
  e.emit(source()); // @source(emit) @miss(silent)
}

// two emitters: the subscriber of one is not called by the other
void emitter_two() {
  final e1 = Emitter();
  final e2 = Emitter();
  e1.on((s) => sink(s)); // @sink(emit-two)
  e2.on((s) => sink(s)); // @sink(none)
  e1.emit(source()); // @source(emit-two) @miss(silent)
  e2.emit('const');
}

class Lazy<T> {
  final T Function() _make;
  T? _v;
  Lazy(this._make);
  T get value => _v ??= _make();
}

void lazy_value() {
  final a = source(); // @source(lazy) @miss(silent)
  final l = Lazy(() => a);
  sink(l.value); // @sink(lazy)
}

class Builder {
  String _acc = '';
  Builder add(String s) {
    _acc += s;
    return this;
  }

  Builder with_(void Function(Builder) f) {
    f(this);
    return this;
  }

  String build() => _acc;
}

void builder_closure() {
  final a = source(); // @source(builder) @miss(depth_limit)
  final b = Builder().with_((x) => x.add(a));
  sink(b.build()); // @sink(builder)
}

// a callback with a default value for a parameter
String withDefault(String s, [String Function(String)? f]) => (f ?? (x) => x)(s);

void default_callback() {
  final a = source(); // @source(default-cb)
  sink(withDefault(a)); // @sink(default-cb)
}

void named_callback() {
  final a = source(); // @source(named-cb)
  sink(withNamed(a, map: (x) => x)); // @sink(named-cb)
}

String withNamed(String s, {required String Function(String) map}) => map(s);

// a getter that returns a function
class Source {
  String Function() get maker => () => source(); // @source(getter-fn) @miss(unmodeled_library_call,unresolved_callee)
}

void getter_function() {
  sink(Source().maker()); // @sink(getter-fn)
}

// an extension method that takes a callback
extension Let<T> on T {
  R let<R>(R Function(T) f) => f(this);
}

void extension_let() {
  final a = source(); // @source(ext-let)
  sink(a.let((x) => x)); // @sink(ext-let)
}

void extension_let_const() {
  final a = source(); // @source(ext-let-const)
  sink(a.let((x) => 'fixed')); // @sink(none)
  sink(a); // @sink(ext-let-const)
}
