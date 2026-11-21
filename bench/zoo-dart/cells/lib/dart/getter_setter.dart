// @cell(F/dart/getter-setter)
// @shape: a value is given to a setter and read from the getter of the same name (the decoy has two properties)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/getter-setter/vuln', vuln);
  zooCase('F/dart/getter-setter/fixed', fixed);
  zooCase('F/dart/getter-setter/decoy', decoy);
}

class Box {
  String _v = '';
  set value(String s) {
    _v = s;
  }

  String get value => _v;
}

void vuln() { // @vuln
  final b = Box();
  b.value = src('dart-getter-setter-vuln'); // @source(dart-getter-setter-vuln)
  sinkCmd(b.value); // @sink(dart-getter-setter-vuln)
}

void fixed() { // @fixed
  final b = Box();
  b.value = cleanCmd(src('dart-getter-setter-fixed'));
  sinkCmd(b.value); // @sink(none)
}

class TwoBoxes {
  String _a = '';
  String _b = '';
  set first(String s) {
    _a = s;
  }

  set second(String s) {
    _b = s;
  }

  String get first => _a;
  String get second => _b;
}

void decoy() { // @decoy
  final b = TwoBoxes();
  b.first = src('dart-getter-setter-decoy'); // @source(dart-getter-setter-decoy)
  b.second = 'constant';
  sinkCmd(b.second); // @sink(none) @nosink(dart-getter-setter-decoy)
  sinkSql(b.first); // @sink(dart-getter-setter-decoy)
}
