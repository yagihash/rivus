// A helper in another package: the closure is given from the package that uses it.
R withIn<T, R>(T value, R Function(T) f) => f(value);

void each<T>(List<T> items, void Function(T) f) {
  for (final i in items) {
    f(i);
  }
}

class Hub {
  final List<void Function(String)> _subs = [];
  void on(void Function(String) f) => _subs.add(f);
  void emit(String s) {
    for (final f in _subs) {
      f(s);
    }
  }
}
