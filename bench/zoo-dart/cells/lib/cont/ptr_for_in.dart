// @cell(F/cont/ptr-for-in)
// @shape: an element taken by for-in is written through, and the same element is read back through the original collection
import 'package:zoo_mark/zoo_mark.dart';

class Box {
  String v = '';
}

void register() {
  zooCase('F/cont/ptr-for-in/vuln', vuln);
  zooCase('F/cont/ptr-for-in/fixed', fixed);
  zooCase('F/cont/ptr-for-in/decoy', decoy);
}

void vuln() { // @vuln
  final boxes = [Box()];
  for (final b in boxes) {
    b.v = src('cont-ptr-for-in-vuln'); // @source(cont-ptr-for-in-vuln) @miss(silent) @ref(#172)
  }
  sinkCmd(boxes[0].v); // @sink(cont-ptr-for-in-vuln)
}

void fixed() { // @fixed
  final boxes = [Box()];
  for (final b in boxes) {
    b.v = cleanCmd(src('cont-ptr-for-in-fixed'));
  }
  sinkCmd(boxes[0].v); // @sink(none)
}

void decoy() { // @decoy
  final boxes = [Box()];
  final others = [Box()];
  for (final b in boxes) {
    b.v = src('cont-ptr-for-in-decoy'); // @source(cont-ptr-for-in-decoy) @miss(silent) @ref(#172)
  }
  sinkCmd(others[0].v); // @sink(none)
  sinkSql(boxes[0].v); // @sink(cont-ptr-for-in-decoy)
}
