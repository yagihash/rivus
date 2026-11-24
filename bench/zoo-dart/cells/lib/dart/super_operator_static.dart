// @cell(F/dart/super-operator-static)
// @na(go: no operator methods or super)
// @shape: super + o, super == o, super[i], super[i] = v and super[i] += v run the operator of the direct superclass, and not the one that a subclass (the one that is calling) overrides it with
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-operator-static/vuln', vuln);
  zooCase('F/dart/super-operator-static/vuln2', vuln2);
  zooCase('F/dart/super-operator-static/vuln3', vuln3);
  zooCase('F/dart/super-operator-static/vuln4', vuln4);
  zooCase('F/dart/super-operator-static/vuln5', vuln5);
  zooCase('F/dart/super-operator-static/vuln6', vuln6);
  zooCase('F/dart/super-operator-static/fixed', fixed);
  zooCase('F/dart/super-operator-static/decoy', decoy);
}


class Base0 {
  String data = '';

  Base0 operator +(Base0 o) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln)
    return this;
  }
}

class Sub0 extends Base0 {
  void run(Base0 o) {
    final r = super + o;
    r.data;
  }
}

void vuln() { // @vuln
  final a = Sub0();
  a.data = src('dart-super-operator-static-vuln'); // @source(dart-super-operator-static-vuln)
  a.run(a);
}

class Base1 {
  String data = '';

  @override
  bool operator ==(Object o) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln2)
    return true;
  }
}

class Sub1 extends Base1 {
  void run(Base1 o) {
    final r = super == o;
    r.toString();
  }
}

void vuln2() { // @vuln
  final a = Sub1();
  a.data = src('dart-super-operator-static-vuln2'); // @source(dart-super-operator-static-vuln2)
  a.run(a);
}

class Base2 {
  String data = '';

  String operator [](int i) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln3)
    return '';
  }
}

class Sub2 extends Base2 {
  void run(Base2 o) {
    final r = super[0];
    r.length;
  }
}

void vuln3() { // @vuln
  final a = Sub2();
  a.data = src('dart-super-operator-static-vuln3'); // @source(dart-super-operator-static-vuln3)
  a.run(a);
}

class Base3 {
  String data = '';

  void operator []=(int i, String s) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln4)
  }
}

class Sub3 extends Base3 {
  void run(Base3 o) {
    super[0] = 'v';
  }
}

void vuln4() { // @vuln
  final a = Sub3();
  a.data = src('dart-super-operator-static-vuln4'); // @source(dart-super-operator-static-vuln4)
  a.run(a);
}

class Base4 {
  String data = '';

  String operator [](int i) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln5)
    return '';
  }

  void operator []=(int i, String s) {
    data.length;
  }
}

class Sub4 extends Base4 {
  void run(Base4 o) {
    super[0] += 'v';
  }
}

void vuln5() { // @vuln
  final a = Sub4();
  a.data = src('dart-super-operator-static-vuln5'); // @source(dart-super-operator-static-vuln5)
  a.run(a);
}

class Base5 {
  String data = '';

  String operator [](int i) => '';

  void operator []=(int i, String s) {
    sinkCmd(data); // @sink(dart-super-operator-static-vuln6)
  }
}

class Sub5 extends Base5 {
  void run(Base5 o) {
    super[0] += 'v';
  }
}

void vuln6() { // @vuln
  final a = Sub5();
  a.data = src('dart-super-operator-static-vuln6'); // @source(dart-super-operator-static-vuln6)
  a.run(a);
}

class BaseF {
  String data = '';

  String operator [](int i) {
    sinkCmd(data); // @sink(none)
    return '';
  }
}

class SubF extends BaseF {
  void run() {
    final r = super[0];
    r.length;
  }
}

void fixed() { // @fixed
  final a = SubF();
  a.data = cleanCmd(src('dart-super-operator-static-fixed'));
  a.run();
}

class BaseD {
  String data = '';
  String _v = '';

  BaseD operator +(BaseD o) => this;

  @override
  bool operator ==(Object o) => true;

  String operator [](int i) => _v;

  void operator []=(int i, String s) {
    _v = s;
  }
}

class SubD extends BaseD {
  @override
  SubD operator +(BaseD o) {
    sinkCmd(data); // @sink(none) @nosink(dart-super-operator-static-decoy)
    return this;
  }

  @override
  bool operator ==(Object o) {
    sinkCmd(data); // @sink(none) @nosink(dart-super-operator-static-decoy)
    return true;
  }

  @override
  String operator [](int i) {
    sinkCmd(data); // @sink(none) @nosink(dart-super-operator-static-decoy)
    return '';
  }

  @override
  void operator []=(int i, String s) {
    sinkCmd(data); // @sink(none) @nosink(dart-super-operator-static-decoy)
  }

  void runSuper(SubD o) {
    final a = super + o;
    a.data;
    final b = super == o;
    b.toString();
    final c = super[0];
    c.length;
    super[0] = 'v';
    super[0] += 'w';
  }
}

void decoy() { // @decoy
  final tainted = SubD();
  tainted.data = src('dart-super-operator-static-decoy'); // @source(dart-super-operator-static-decoy)
  tainted.runSuper(tainted);
  final clean = SubD();
  clean.data = 'constant';
  final a = clean + clean;
  a.data;
  final b = clean == clean;
  b.toString();
  final c = clean[0];
  c.length;
  clean[0] = 'v';
  sinkSql(tainted.data); // @sink(dart-super-operator-static-decoy)
}
