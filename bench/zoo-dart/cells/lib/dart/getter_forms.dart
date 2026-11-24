// @cell(F/dart/getter-forms)
// @na(go: no getters and setters)
// @shape: a getter body is reached by every form of read: a.x, a.x += v (it reads before it writes), this.x, x and super.x
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/getter-forms/vuln', vuln);
  zooCase('F/dart/getter-forms/vuln2', vuln2);
  zooCase('F/dart/getter-forms/vuln3', vuln3);
  zooCase('F/dart/getter-forms/vuln4', vuln4);
  zooCase('F/dart/getter-forms/vuln5', vuln5);
  zooCase('F/dart/getter-forms/fixed', fixed);
  zooCase('F/dart/getter-forms/decoy', decoy);
}


class Cls0 {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(dart-getter-forms-vuln)
    return _x;
  }
}

void vuln() { // @vuln
  final a = Cls0();
  a.data = src('dart-getter-forms-vuln'); // @source(dart-getter-forms-vuln)
  final r = a.x;
  r.length;
}

class Cls1 {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(dart-getter-forms-vuln2)
    return _x;
  }

  set x(String s) {
    _x = s;
  }
}

void vuln2() { // @vuln
  final a = Cls1();
  a.data = src('dart-getter-forms-vuln2'); // @source(dart-getter-forms-vuln2)
  a.x += 'v';
}

class Cls2 {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(dart-getter-forms-vuln3)
    return _x;
  }

  void readThis() {
    final r = this.x;
    r.length;
  }
}

void vuln3() { // @vuln
  final a = Cls2();
  a.data = src('dart-getter-forms-vuln3'); // @source(dart-getter-forms-vuln3)
  a.readThis();
}

class Cls3 {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(dart-getter-forms-vuln4)
    return _x;
  }

  void readImplicit() {
    final r = x;
    r.length;
  }
}

void vuln4() { // @vuln
  final a = Cls3();
  a.data = src('dart-getter-forms-vuln4'); // @source(dart-getter-forms-vuln4)
  a.readImplicit();
}

class Cls4 {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(dart-getter-forms-vuln5)
    return _x;
  }
}

class Sub4 extends Cls4 {
  void readSuper() {
    final r = super.x;
    r.length;
  }
}

void vuln5() { // @vuln
  final a = Sub4();
  a.data = src('dart-getter-forms-vuln5'); // @source(dart-getter-forms-vuln5)
  a.readSuper();
}

class ClsFixed {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(none)
    return _x;
  }
}

void fixed() { // @fixed
  final a = ClsFixed();
  a.data = cleanCmd(src('dart-getter-forms-fixed'));
  final r = a.x;
  r.length;
}

class ClsDecoy {
  String data = '';
  String _x = '';

  String get x {
    sinkCmd(data); // @sink(none) @nosink(dart-getter-forms-decoy)
    return _x;
  }

  set x(String s) {
    _x = s;
  }
}

void decoy() { // @decoy
  final tainted = ClsDecoy();
  tainted.data = src('dart-getter-forms-decoy'); // @source(dart-getter-forms-decoy)
  tainted.x = 'v';
  final clean = ClsDecoy();
  clean.data = 'constant';
  final r = clean.x;
  r.length;
  sinkSql(tainted.data); // @sink(dart-getter-forms-decoy)
}
