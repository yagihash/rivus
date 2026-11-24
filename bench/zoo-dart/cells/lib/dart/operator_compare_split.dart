// @cell(F/dart/operator-compare-split)
// @na(go: no operator methods)
// @shape: the operators < and <= are two functions; a <= b runs the second and never the first, a < b runs the first (the decoy compares with <= a tainted object, and with < a clean one)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/operator-compare-split/vuln', vuln);
  zooCase('F/dart/operator-compare-split/fixed', fixed);
  zooCase('F/dart/operator-compare-split/decoy', decoy);
}

class Gauge {
  String data = '';

  bool operator <(Gauge o) {
    sinkCmd(data); // @sink(dart-operator-compare-split-vuln)
    return true;
  }

  bool operator <=(Gauge o) => o.data.isEmpty;
}

void vuln() { // @vuln
  final g = Gauge();
  g.data = src('dart-operator-compare-split-vuln'); // @source(dart-operator-compare-split-vuln)
  final o = Gauge();
  g <= o;
  g < o;
}

class GaugeFixed {
  String data = '';

  bool operator <(GaugeFixed o) {
    sinkCmd(data); // @sink(none)
    return true;
  }

  bool operator <=(GaugeFixed o) => o.data.isEmpty;
}

void fixed() { // @fixed
  final g = GaugeFixed();
  g.data = cleanCmd(src('dart-operator-compare-split-fixed'));
  final o = GaugeFixed();
  g <= o;
  g < o;
}

class GaugeDecoy {
  String data = '';

  bool operator <(GaugeDecoy o) {
    sinkCmd(data); // @sink(none) @nosink(dart-operator-compare-split-decoy)
    return true;
  }

  bool operator <=(GaugeDecoy o) => o.data.isEmpty;
}

void decoy() { // @decoy
  final tainted = GaugeDecoy();
  tainted.data = src('dart-operator-compare-split-decoy'); // @source(dart-operator-compare-split-decoy)
  final clean = GaugeDecoy();
  clean.data = 'constant';
  final o = GaugeDecoy();
  tainted <= o;
  clean < o;
  sinkSql(tainted.data); // @sink(dart-operator-compare-split-decoy)
}
