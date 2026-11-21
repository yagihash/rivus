// @cell(F/cont/slice)
// @shape: a value is an element of a list literal (a slice in Go), and the element is read by its index (the decoy reads the other element)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/cont/slice/vuln', vuln);
  zooCase('F/cont/slice/fixed', fixed);
  zooCase('F/cont/slice/decoy', decoy);
}

void vuln() { // @vuln
  final l = [src('cont-slice-vuln'), 'constant']; // @source(cont-slice-vuln)
  sinkCmd(l[0]); // @sink(cont-slice-vuln)
}

void fixed() { // @fixed
  final l = [cleanCmd(src('cont-slice-fixed')), 'constant'];
  sinkCmd(l[0]); // @sink(none)
}

void decoy() { // @decoy
  final l = [src('cont-slice-decoy'), 'constant']; // @source(cont-slice-decoy)
  sinkCmd(l[1]); // @sink(none) @fp @nosink(cont-slice-decoy) @fp @ref(KEYED-MAP)
  sinkSql(l[0]); // @sink(cont-slice-decoy)
}
