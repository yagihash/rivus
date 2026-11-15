import 'lib.dart';

// Dart 3 patterns: the values they take apart and bind.

sealed class Shape {}

class Circle extends Shape {
  Circle(this.label);
  final String label;
}

class Square extends Shape {
  Square(this.label);
  final String label;
}

void switchObject() {
  final Shape s = Circle(source()); // @source(pat-obj)
  switch (s) {
    case Circle(label: final l):
      sink(l); // @sink(pat-obj)
    case Square():
      sink('x'); // @sink(none)
  }
}

void ifCaseMap() {
  final data = {'name': source()}; // @source(pat-map)
  if (data case {'name': String n}) {
    sink(n); // @sink(pat-map)
  }
}

void recordDestructuring() {
  final r = (source(), 1); // @source(pat-rec)
  final (a, b) = r;
  sink(a); // @sink(pat-rec)
}

void listDestructuring() {
  final xs = [source(), 'b']; // @source(pat-list)
  final [first, _] = xs;
  sink(first); // @sink(pat-list)
}

String switchExpression(int k) {
  final t = source(); // @source(pat-switchexpr)
  final r = switch (k) {
    1 => t,
    _ => 'safe',
  };
  sink(r); // @sink(pat-switchexpr)
  return r;
}

void namedRecord() {
  final r = (name: source(), n: 1); // @source(pat-recn)
  sink(r.name); // @sink(pat-recn)
}

void clean(Shape s) {
  final x = switch (s) {
    Circle(label: final l) => l,
    Square() => 'square',
  };
  sink('fixed'); // @sink(none)
  print(x);
}
