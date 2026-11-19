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

// --- #28 (rest): the implicit super() runs the field initializers of the base class

class BaseImp {
  final String v = source(); // @source(is-28a)
  final String w = source(); // @source(is-28d)
}

class DerivedImp extends BaseImp {
  DerivedImp(this.x);
  final int x;
}

class DerivedNoCtor extends BaseImp {}

void implicitSuper() {
  sink(DerivedImp(1).v); // @sink(is-28a)
  sink(DerivedNoCtor().w); // @sink(is-28d)
}

class BaseBody {
  BaseBody() {
    v = source(); // @source(is-28b)
  }
  late String v;
}

class DerivedBody extends BaseBody {
  DerivedBody(this.n);
  final int n;
}

void implicitSuperBody() {
  sink(DerivedBody(2).v); // @sink(is-28b)
}

// --- #28 (rest): a super parameter together with an explicit super.named() -

class BaseNamed {
  BaseNamed.named(this.v);
  final String v;
}

class DerivedNamed extends BaseNamed {
  DerivedNamed(super.v) : super.named();
}

void superParamWithNamed() {
  final s = source(); // @source(is-28c)
  sink(DerivedNamed(s).v); // @sink(is-28c)
}

// --- #29 (rest): the field initializers of a mixin run in the constructor of the class that uses it

mixin Tagged {
  final String tag = source(); // @source(is-29m)
}

class Doc with Tagged {
  Doc(this.id);
  final int id;
}

class DocNoCtor with Tagged {}

void mixinField() {
  sink(Doc(1).tag); // @sink(is-29m)
}

mixin Tagged2 {
  final String tag2 = source(); // @source(is-29n)
}

class DocNoCtor2 with Tagged2 {}

void mixinFieldNoCtor() {
  sink(DocNoCtor2().tag2); // @sink(is-29n)
}

// --- #29 (rest): an enum field with an initializer ---------------------------

enum Level {
  low(1),
  high(2);

  const Level(this.n);
  final int n;
  final String label = 'level';
}

String levelLabel() => Level.low.label;

// --- #62: an assert in the initializer list of a constructor runs like one in a body

bool _auditInit(String p) {
  sink(p); // @sink(is-62)
  return true;
}

class AssertInit {
  final String s;
  AssertInit(String p) : assert(true, 'bad ${_auditInit(p)}'), s = p;
}

void assertInitializer() {
  AssertInit(source()); // @source(is-62)
}
