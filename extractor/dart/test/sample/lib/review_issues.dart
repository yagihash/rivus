import 'lib.dart';

// The repro code of the issues that the daily review made (yagihash/rivus #27-#36).

// --- #27: a case that is not empty does not fall into the next one (Dart 3) -

void switchNoFallthrough(int k) {
  var q = 'safe';
  switch (k) {
    case 1:
      q = source(); // @source(is-27)
    case 2:
      q = 'fixed';
  }
  sink(q); // @sink(is-27)
}

// --- #28: a super parameter reaches the field of the base class ------------

class BaseP {
  BaseP(this.v);
  final String v;
}

class DerivedP extends BaseP {
  DerivedP(super.v);
}

void superParameter() {
  final s = source(); // @source(is-28)
  sink(DerivedP(s).v); // @sink(is-28)
}

// --- #29: the initializer of a field of a class that has no constructor ----

class Holder {
  final String v = source(); // @source(is-29)
}

void holderField() {
  sink(Holder().v); // @sink(is-29)
}

// --- #31: ??= assigns only when the variable is null -----------------------

void nullAwareAssign() {
  String? s = source(); // @source(is-31)
  s ??= 'default';
  sink(s); // @sink(is-31)
}

// --- #33: a call in a cascade that has no target is a call of this ---------

class Page {
  Page(this.title);
  final String title;
  String _header() => '<h1>$title</h1>';
  String render() => (StringBuffer()..write(_header())).toString();
}

void cascadeImplicitThis() {
  final s = source(); // @source(is-33)
  sink(Page(s).render()); // @sink(is-33)
}

// --- #34: the initializer list reads a this.x parameter --------------------

class Req {
  Req(this.raw) : path = raw.trim();
  final String raw;
  final String path;
}

void initializerListReadsParameter() {
  final s = source(); // @source(is-34)
  sink(Req(s).path); // @sink(is-34)
}

// --- #35: an operator that is used with an assignment ----------------------

class Acc {
  Acc(this.s);
  String s;
  Acc operator +(String o) => Acc(s + o);
}

void compoundOperator() {
  var a = Acc('x');
  final t = source(); // @source(is-35)
  a += t;
  sink(a.s); // @sink(is-35)
}

void plainOperator() {
  final t = source(); // @source(is-35b)
  final b = Acc('x') + t;
  sink(b.s); // @sink(is-35b)
}
