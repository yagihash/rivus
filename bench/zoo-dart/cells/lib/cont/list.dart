// @cell(F/cont/list)
// @shape: a value is an element of a list literal, and the element is read by its index (the decoy reads the other element)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/cont/list/vuln', vuln);
  zooCase('F/cont/list/fixed', fixed);
  zooCase('F/cont/list/decoy', decoy);
}

void vuln() { // @vuln
  final l = [src('cont-list-vuln'), 'constant']; // @source(cont-list-vuln)
  sinkCmd(l[0]); // @sink(cont-list-vuln)
}

void fixed() { // @fixed
  final l = [cleanCmd(src('cont-list-fixed')), 'constant'];
  sinkCmd(l[0]); // @sink(none)
}

void decoy() { // @decoy
  final l = [src('cont-list-decoy'), 'constant']; // @source(cont-list-decoy)
  sinkCmd(l[1]); // @sink(none) @fp @nosink(cont-list-decoy) @fp @ref(KEYED-MAP)
  sinkSql(l[0]); // @sink(cont-list-decoy)
}
