/// What the dependency writes to (the role of io.Writer: the interface is the dependency's, so that the
/// writers of the project that implement it are those of one cell only).
abstract class Writer {
  void write(String data);
}

/// A writer that hands what is written to the writer that it wraps (the shape of the logging and
/// compressing wrappers of a dependency: its write calls the write of what it wraps).
Writer wrap(Writer w) => _Wrapper(w);

class _Wrapper implements Writer {
  _Wrapper(this._w);
  final Writer _w;

  @override
  void write(String data) {
    _w.write(data);
  }
}

/// A middleware of a dependency: the function that it returns calls the function that it was given (the shape of the
/// wrappers of a server framework: `wrapFn(handler)` is called where the handler would have been).
String Function(String) wrapFn(String Function(String) h) => (x) => h(x);
