// @cell(F/dart/setter-forms)
// @na(go: no getters and setters)
// @shape: a setter body is reached by every form of assignment: a.x = v, a.x += v, a.x ??= v, a..x = v, this.x = v, x = v, super.x = v, and a subclass setter that calls super.x = v
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/setter-forms/vuln', vuln);
  zooCase('F/dart/setter-forms/vuln2', vuln2);
  zooCase('F/dart/setter-forms/vuln3', vuln3);
  zooCase('F/dart/setter-forms/vuln4', vuln4);
  zooCase('F/dart/setter-forms/vuln5', vuln5);
  zooCase('F/dart/setter-forms/vuln6', vuln6);
  zooCase('F/dart/setter-forms/vuln7', vuln7);
  zooCase('F/dart/setter-forms/vuln8', vuln8);
  zooCase('F/dart/setter-forms/fixed', fixed);
  zooCase('F/dart/setter-forms/decoy', decoy);
}


class Cls0 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln)
  }
}

void vuln() { // @vuln
  final a = Cls0();
  a.data = src('dart-setter-forms-vuln'); // @source(dart-setter-forms-vuln)
  a.x = 'v';
}

class Cls1 {
  String data = '';
  String? _x;

  String get x => _x ?? '';

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln2)
  }
}

void vuln2() { // @vuln
  final a = Cls1();
  a.data = src('dart-setter-forms-vuln2'); // @source(dart-setter-forms-vuln2)
  a.x += 'v';
}

class Cls2 {
  String data = '';
  String? _x;

  String? get x => _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln3)
  }
}

void vuln3() { // @vuln
  final a = Cls2();
  a.data = src('dart-setter-forms-vuln3'); // @source(dart-setter-forms-vuln3)
  a.x ??= 'v';
}

class Cls3 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln4)
  }
}

void vuln4() { // @vuln
  final a = Cls3();
  a.data = src('dart-setter-forms-vuln4'); // @source(dart-setter-forms-vuln4)
  a..x = 'v';
}

class Cls4 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln5)
  }

  void assignThis() {
    this.x = 'v';
  }
}

void vuln5() { // @vuln
  final a = Cls4();
  a.data = src('dart-setter-forms-vuln5'); // @source(dart-setter-forms-vuln5)
  a.assignThis();
}

class Cls5 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln6)
  }

  void assignImplicit() {
    x = 'v';
  }
}

void vuln6() { // @vuln
  final a = Cls5();
  a.data = src('dart-setter-forms-vuln6'); // @source(dart-setter-forms-vuln6)
  a.assignImplicit();
}

class Cls6 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln7)
  }
}

class Sub6 extends Cls6 {
  void assignSuper() {
    super.x = 'v';
  }
}

void vuln7() { // @vuln
  final a = Sub6();
  a.data = src('dart-setter-forms-vuln7'); // @source(dart-setter-forms-vuln7)
  a.assignSuper();
}

class Cls7 {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(dart-setter-forms-vuln8)
  }
}

class Sub7 extends Cls7 {
  set x(String s) {
    super.x = s;
  }
}

void vuln8() { // @vuln
  final a = Sub7();
  a.data = src('dart-setter-forms-vuln8'); // @source(dart-setter-forms-vuln8)
  a.x = 'v';
}

class ClsFixed {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(none)
  }
}

void fixed() { // @fixed
  final a = ClsFixed();
  a.data = cleanCmd(src('dart-setter-forms-fixed'));
  a.x = 'v';
}

class ClsDecoy {
  String data = '';
  String? _x;

  set x(String s) {
    _x = s;
    sinkCmd(data); // @sink(none) @nosink(dart-setter-forms-decoy)
  }
}

void decoy() { // @decoy
  final tainted = ClsDecoy();
  tainted.data = src('dart-setter-forms-decoy'); // @source(dart-setter-forms-decoy)
  final clean = ClsDecoy();
  clean.data = 'constant';
  clean.x = 'v';
  sinkSql(tainted.data); // @sink(dart-setter-forms-decoy)
}
