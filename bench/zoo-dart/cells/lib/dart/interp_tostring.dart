// @cell(F/dart/interp-tostring)
// @na(go: no implicit String() call in a concatenation)
// @shape: an object is interpolated into a string, which calls its toString, and the text is sunk (the decoy's toString returns the other field)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/interp-tostring/vuln', vuln);
  zooCase('F/dart/interp-tostring/fixed', fixed);
  zooCase('F/dart/interp-tostring/decoy', decoy);
}

class Wrapper {
  Wrapper(this.v);
  final String v;
  @override
  String toString() => v;
}

void vuln() { // @vuln
  final w = Wrapper(src('dart-interp-tostring-vuln')); // @source(dart-interp-tostring-vuln)
  sinkCmd('run $w'); // @sink(dart-interp-tostring-vuln)
}

class WrapperFixed {
  WrapperFixed(this.v);
  final String v;
  @override
  String toString() => v;
}

void fixed() { // @fixed
  final w = WrapperFixed(cleanCmd(src('dart-interp-tostring-fixed')));
  sinkCmd('run $w'); // @sink(none)
}

class WrapperDecoy {
  WrapperDecoy(this.secret, this.label);
  final String secret;
  final String label;
  @override
  String toString() => label;
}

void decoy() { // @decoy
  final w = WrapperDecoy(src('dart-interp-tostring-decoy'), 'constant'); // @source(dart-interp-tostring-decoy)
  sinkCmd('run $w'); // @sink(none) @fp @nosink(dart-interp-tostring-decoy) @fp @ref(R-16)
  sinkSql(w.secret); // @sink(dart-interp-tostring-decoy)
}
