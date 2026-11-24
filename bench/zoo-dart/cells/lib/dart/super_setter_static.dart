// @cell(F/dart/super-setter-static)
// @na(go: no setters or super)
// @shape: super.x = v runs the setter of the direct superclass and not the setter of the subclass that is calling (the decoy assigns super.x of a tainted object, and x of a clean one)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-setter-static/vuln', vuln);
  zooCase('F/dart/super-setter-static/fixed', fixed);
  zooCase('F/dart/super-setter-static/decoy', decoy);
}

class Base {
  String data = '';
  String last = '';

  set x(String s) {
    last = s;
    sinkCmd(data); // @sink(dart-super-setter-static-vuln)
  }
}

class Sub extends Base {
  @override
  set x(String s) {
    super.x = s;
  }

  void writeSuper() {
    super.x = 'v';
  }
}

void vuln() { // @vuln
  final a = Sub();
  a.data = src('dart-super-setter-static-vuln'); // @source(dart-super-setter-static-vuln)
  a.x = 'v';
  a.writeSuper();
}

class BaseFixed {
  String data = '';
  String last = '';

  set x(String s) {
    last = s;
    sinkCmd(data); // @sink(none)
  }
}

class SubFixed extends BaseFixed {
  @override
  set x(String s) {
    super.x = s;
  }

  void writeSuper() {
    super.x = 'v';
  }
}

void fixed() { // @fixed
  final a = SubFixed();
  a.data = cleanCmd(src('dart-super-setter-static-fixed'));
  a.x = 'v';
  a.writeSuper();
}

class BaseDecoy {
  String data = '';
  String last = '';

  set x(String s) {
    last = s;
  }
}

class SubDecoy extends BaseDecoy {
  @override
  set x(String s) {
    last = s;
    sinkCmd(data); // @sink(none) @nosink(dart-super-setter-static-decoy)
  }

  void writeSuper() {
    super.x = 'v';
  }
}

void decoy() { // @decoy
  final tainted = SubDecoy();
  tainted.data = src('dart-super-setter-static-decoy'); // @source(dart-super-setter-static-decoy)
  tainted.writeSuper();
  final clean = SubDecoy();
  clean.data = 'constant';
  clean.x = 'v';
  sinkSql(tainted.data); // @sink(dart-super-setter-static-decoy)
}
