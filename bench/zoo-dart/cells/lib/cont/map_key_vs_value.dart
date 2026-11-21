// @cell(F/cont/map-key-vs-value)
// @shape: a value is the value of a key of a map literal, and it is read by that key (the decoy reads the other key)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/cont/map-key-vs-value/vuln', vuln);
  zooCase('F/cont/map-key-vs-value/fixed', fixed);
  zooCase('F/cont/map-key-vs-value/decoy', decoy);
}

void vuln() { // @vuln
  final m = {'a': src('cont-map-key-vs-value-vuln'), 'b': 'constant'}; // @source(cont-map-key-vs-value-vuln)
  sinkCmd(m['a']!); // @sink(cont-map-key-vs-value-vuln)
}

void fixed() { // @fixed
  final m = {'a': cleanCmd(src('cont-map-key-vs-value-fixed')), 'b': 'constant'};
  sinkCmd(m['a']!); // @sink(none)
}

void decoy() { // @decoy
  final m = {'a': src('cont-map-key-vs-value-decoy'), 'b': 'constant'}; // @source(cont-map-key-vs-value-decoy)
  sinkCmd(m['b']!); // @sink(none) @fp @nosink(cont-map-key-vs-value-decoy) @fp @ref(KEYED-MAP)
  sinkSql(m['a']!); // @sink(cont-map-key-vs-value-decoy)
}
