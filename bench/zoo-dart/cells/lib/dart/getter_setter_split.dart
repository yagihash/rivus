// @cell(F/dart/getter-setter-split)
// @na(go: no getters and setters; a field is read and written directly)
// @shape: a getter and a setter have the same name; reading the property runs the getter and never the setter, assigning to it runs the setter (the decoy reads the property of a tainted object, and assigns to the property of a clean one)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/getter-setter-split/vuln', vuln);
  zooCase('F/dart/getter-setter-split/fixed', fixed);
  zooCase('F/dart/getter-setter-split/decoy', decoy);
}

class Holder {
  String data = '';
  String _kept = '';

  String get kept => _kept;

  set kept(String s) {
    _kept = s;
    sinkCmd(data); // @sink(dart-getter-setter-split-vuln)
  }
}

void vuln() { // @vuln
  final h = Holder();
  h.data = src('dart-getter-setter-split-vuln'); // @source(dart-getter-setter-split-vuln)
  h.kept = h.kept;
}

class HolderFixed {
  String data = '';
  String _kept = '';

  String get kept => _kept;

  set kept(String s) {
    _kept = s;
    sinkCmd(data); // @sink(none)
  }
}

void fixed() { // @fixed
  final h = HolderFixed();
  h.data = cleanCmd(src('dart-getter-setter-split-fixed'));
  h.kept = h.kept;
}

class HolderDecoy {
  String data = '';
  String _kept = '';

  String get kept => _kept;

  set kept(String s) {
    _kept = s;
    sinkCmd(data); // @sink(none) @nosink(dart-getter-setter-split-decoy)
  }
}

void decoy() { // @decoy
  final tainted = HolderDecoy();
  tainted.data = src('dart-getter-setter-split-decoy'); // @source(dart-getter-setter-split-decoy)
  final r = tainted.kept;
  final clean = HolderDecoy();
  clean.data = 'constant';
  clean.kept = r;
  sinkSql(tainted.data); // @sink(dart-getter-setter-split-decoy)
}
