// @cell(F/dart/operator-forms)
// @na(go: no operator methods)
// @shape: the body of an operator is reached by its own form only: a == b, a[i], a[i] = v, a[i] += v (both [] and []=), a < b, a <= b, a > b, a >= b
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/operator-forms/vuln', vuln);
  zooCase('F/dart/operator-forms/vuln2', vuln2);
  zooCase('F/dart/operator-forms/vuln3', vuln3);
  zooCase('F/dart/operator-forms/vuln4', vuln4);
  zooCase('F/dart/operator-forms/vuln5', vuln5);
  zooCase('F/dart/operator-forms/vuln6', vuln6);
  zooCase('F/dart/operator-forms/vuln7', vuln7);
  zooCase('F/dart/operator-forms/vuln8', vuln8);
  zooCase('F/dart/operator-forms/vuln9', vuln9);
  zooCase('F/dart/operator-forms/fixed', fixed);
  zooCase('F/dart/operator-forms/decoy', decoy);
}


class Cls0 {
  String data = '';
  String _v = '';

  bool operator ==(Object o) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln)
    return true;
  }
}

void vuln() { // @vuln
  final a = Cls0();
  a.data = src('dart-operator-forms-vuln'); // @source(dart-operator-forms-vuln)
  a == Object();
}

class Cls1 {
  String data = '';
  String _v = '';

  String operator [](int i) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln2)
    return _v;
  }
}

void vuln2() { // @vuln
  final a = Cls1();
  a.data = src('dart-operator-forms-vuln2'); // @source(dart-operator-forms-vuln2)
  final r = a[0];
  r.length;
}

class Cls2 {
  String data = '';
  String _v = '';

  void operator []=(int i, String s) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln3)
    _v = s;
  }
}

void vuln3() { // @vuln
  final a = Cls2();
  a.data = src('dart-operator-forms-vuln3'); // @source(dart-operator-forms-vuln3)
  a[0] = 'v';
}

class Cls3 {
  String data = '';
  String _v = '';

  String operator [](int i) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln4)
    return _v;
  }

  void operator []=(int i, String s) {
    _v = s;
  }
}

void vuln4() { // @vuln
  final a = Cls3();
  a.data = src('dart-operator-forms-vuln4'); // @source(dart-operator-forms-vuln4)
  a[0] += 'v';
}

class Cls4 {
  String data = '';
  String _v = '';

  String operator [](int i) => _v;

  void operator []=(int i, String s) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln5)
    _v = s;
  }
}

void vuln5() { // @vuln
  final a = Cls4();
  a.data = src('dart-operator-forms-vuln5'); // @source(dart-operator-forms-vuln5)
  a[0] += 'v';
}

class Cls5 {
  String data = '';
  String _v = '';

  bool operator <(Cls5 o) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln6)
    return true;
  }
}

void vuln6() { // @vuln
  final a = Cls5();
  a.data = src('dart-operator-forms-vuln6'); // @source(dart-operator-forms-vuln6)
  a < Cls5();
}

class Cls6 {
  String data = '';
  String _v = '';

  bool operator <=(Cls6 o) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln7)
    return true;
  }
}

void vuln7() { // @vuln
  final a = Cls6();
  a.data = src('dart-operator-forms-vuln7'); // @source(dart-operator-forms-vuln7)
  a <= Cls6();
}

class Cls7 {
  String data = '';
  String _v = '';

  bool operator >(Cls7 o) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln8)
    return true;
  }
}

void vuln8() { // @vuln
  final a = Cls7();
  a.data = src('dart-operator-forms-vuln8'); // @source(dart-operator-forms-vuln8)
  a > Cls7();
}

class Cls8 {
  String data = '';
  String _v = '';

  bool operator >=(Cls8 o) {
    sinkCmd(data); // @sink(dart-operator-forms-vuln9)
    return true;
  }
}

void vuln9() { // @vuln
  final a = Cls8();
  a.data = src('dart-operator-forms-vuln9'); // @source(dart-operator-forms-vuln9)
  a >= Cls8();
}

class Cls20 {
  String data = '';
  String _v = '';

  bool operator <(Cls20 o) {
    sinkCmd(data); // @sink(none)
    return true;
  }
}

void fixed() { // @fixed
  final a = Cls20();
  a.data = cleanCmd(src('dart-operator-forms-fixed'));
  a < Cls20();
}

class Cls21 {
  String data = '';
  String _v = '';

  bool operator <(Cls21 o) {
    sinkCmd(data); // @sink(none) @nosink(dart-operator-forms-decoy)
    return true;
  }
}

void decoy() { // @decoy
  final tainted = Cls21();
  tainted.data = src('dart-operator-forms-decoy'); // @source(dart-operator-forms-decoy)
  final clean = Cls21();
  clean.data = 'constant';
  clean < Cls21();
  sinkSql(tainted.data); // @sink(dart-operator-forms-decoy)
}
