// @cell(F/cont/struct-whole-to-sink)
// @shape: an object with a field that holds the value is given whole to a sink (sinkAny takes the text of the value, which is built from its fields)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/cont/struct-whole-to-sink/vuln', vuln);
  zooCase('F/cont/struct-whole-to-sink/fixed', fixed);
  zooCase('F/cont/struct-whole-to-sink/decoy', decoy);
}

class Pair {
  Pair(this.a, this.b);
  final String a;
  final String b;

  @override
  String toString() => 'Pair($a, $b)';
}

/// cleanAll sanitizes for every class: sinkAny judges the text by all of them.
String cleanAll(String s) => cleanPath(cleanHtml(cleanSql(cleanCmd(s))));

void vuln() { // @vuln
  final p = Pair(src('cont-struct-whole-to-sink-vuln'), 'constant'); // @source(cont-struct-whole-to-sink-vuln)
  sinkAny(p); // @sink(cont-struct-whole-to-sink-vuln)
}

void fixed() { // @fixed
  final p = Pair(cleanAll(src('cont-struct-whole-to-sink-fixed')), 'constant');
  sinkAny(p); // @sink(none)
}

void decoy() { // @decoy
  final t = src('cont-struct-whole-to-sink-decoy'); // @source(cont-struct-whole-to-sink-decoy)
  final p = Pair('constant', 'constant');
  sinkAny(p); // @sink(none) @nosink(cont-struct-whole-to-sink-decoy)
  sinkCmd(t); // @sink(cont-struct-whole-to-sink-decoy)
}
