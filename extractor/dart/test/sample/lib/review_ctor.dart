import 'lib.dart';

// What a review of the extractor asked for (REVIEW-2026-11-19): constructors.

// --- DART-2: a factory constructor makes what its body returns --------------

class Person {
  Person._(this.name);
  factory Person.fromText(String t) => Person._(t);
  final String name;
}

void factoryConstructor() {
  final s = source(); // @source(rv-factory)
  sink(Person.fromText(s).name); // @sink(rv-factory)
}

abstract class Shape {
  factory Shape(String n) = _Circle;
  String get n;
}

class _Circle implements Shape {
  _Circle(this.n);
  @override
  final String n;
}

void redirectingFactory() {
  final s = source(); // @source(rv-redirect)
  sink(Shape(s).n); // @sink(rv-redirect)
}

// --- DART-3: super(...) and a redirecting constructor -----------------------

class Base {
  Base(this.v);
  final String v;
}

class Child extends Base {
  Child(String v) : super(v);
}

class Twice extends Base {
  Twice.one(String v) : this.two(v);
  Twice.two(String v) : super(v);
}

void superCall() {
  final s = source(); // @source(rv-super)
  sink(Child(s).v); // @sink(rv-super)
}

void redirectingCall() {
  final s = source(); // @source(rv-this)
  sink(Twice.one(s).v); // @sink(rv-this)
}

