// @cell(F/cont/map)
// @shape: a value is the value of a key of a map literal, and it is read by that key (the decoy reads the other key)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/cont/map/vuln', vuln);
  zooCase('F/cont/map/fixed', fixed);
  zooCase('F/cont/map/decoy', decoy);
}

void vuln() { // @vuln
  final m = {'a': src('cont-map-vuln'), 'b': 'constant'}; // @source(cont-map-vuln)
  sinkCmd(m['a']!); // @sink(cont-map-vuln)
}

void fixed() { // @fixed
  final m = {'a': cleanCmd(src('cont-map-fixed')), 'b': 'constant'};
  sinkCmd(m['a']!); // @sink(none)
}

void decoy() { // @decoy
  final m = {'a': src('cont-map-decoy'), 'b': 'constant'}; // @source(cont-map-decoy)
  sinkCmd(m['b']!); // @sink(none) @fp @nosink(cont-map-decoy) @fp @ref(KEYED-MAP)
  sinkSql(m['a']!); // @sink(cont-map-decoy)
}
