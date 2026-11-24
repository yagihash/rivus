// @cell(F/dart/operator-index-split)
// @na(go: no operator methods; an index is the builtin one)
// @shape: the operators [] and []= are two functions; a[i] runs the first and never the second, a[i] = v runs the second (the decoy reads an element of a tainted object, and assigns to an element of a clean one)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/operator-index-split/vuln', vuln);
  zooCase('F/dart/operator-index-split/fixed', fixed);
  zooCase('F/dart/operator-index-split/decoy', decoy);
}

class Box {
  String data = '';
  String _v = '';

  String operator [](int i) => _v;

  void operator []=(int i, String s) {
    _v = s;
    sinkCmd(data); // @sink(dart-operator-index-split-vuln)
  }
}

void vuln() { // @vuln
  final b = Box();
  b.data = src('dart-operator-index-split-vuln'); // @source(dart-operator-index-split-vuln)
  b[0] = b[0];
}

class BoxFixed {
  String data = '';
  String _v = '';

  String operator [](int i) => _v;

  void operator []=(int i, String s) {
    _v = s;
    sinkCmd(data); // @sink(none)
  }
}

void fixed() { // @fixed
  final b = BoxFixed();
  b.data = cleanCmd(src('dart-operator-index-split-fixed'));
  b[0] = b[0];
}

class BoxDecoy {
  String data = '';
  String _v = '';

  String operator [](int i) => _v;

  void operator []=(int i, String s) {
    _v = s;
    sinkCmd(data); // @sink(none) @nosink(dart-operator-index-split-decoy)
  }
}

void decoy() { // @decoy
  final tainted = BoxDecoy();
  tainted.data = src('dart-operator-index-split-decoy'); // @source(dart-operator-index-split-decoy)
  final r = tainted[0];
  final clean = BoxDecoy();
  clean.data = 'constant';
  clean[0] = r;
  sinkSql(tainted.data); // @sink(dart-operator-index-split-decoy)
}
