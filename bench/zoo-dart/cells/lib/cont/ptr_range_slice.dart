// @cell(F/cont/ptr-range-slice)
// @shape: an element taken by for-in is written through, and the same element is read back through the original collection
import 'package:zoo_mark/zoo_mark.dart';

class Box {
  String v = '';
}

void register() {
  zooCase('F/cont/ptr-range-slice/vuln', vuln);
  zooCase('F/cont/ptr-range-slice/fixed', fixed);
  zooCase('F/cont/ptr-range-slice/decoy', decoy);
}

void vuln() { // @vuln
  final boxes = [Box()];
  for (final b in boxes) {
    b.v = src('cont-ptr-range-slice-vuln'); // @source(cont-ptr-range-slice-vuln) @miss(silent) @ref(#172,PARITY)
  }
  sinkCmd(boxes[0].v); // @sink(cont-ptr-range-slice-vuln)
}

void fixed() { // @fixed
  final boxes = [Box()];
  for (final b in boxes) {
    b.v = cleanCmd(src('cont-ptr-range-slice-fixed'));
  }
  sinkCmd(boxes[0].v); // @sink(none)
}

void decoy() { // @decoy
  final boxes = [Box()];
  final others = [Box()];
  for (final b in boxes) {
    b.v = src('cont-ptr-range-slice-decoy'); // @source(cont-ptr-range-slice-decoy) @miss(silent) @ref(#172,PARITY)
  }
  sinkCmd(others[0].v); // @sink(none)
  sinkSql(boxes[0].v); // @sink(cont-ptr-range-slice-decoy)
}
