// @cell(F/clos/to-sort)
// @shape: a comparison function that is given to a sort of the core library (List.sort, Iterable.reduce) is called with the elements: what the list holds reaches the sink in the function
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-sort/vuln', vuln);
  zooCase('F/clos/to-sort/vuln2', vuln2);
  zooCase('F/clos/to-sort/fixed', fixed);
  zooCase('F/clos/to-sort/decoy', decoy);
}

void vuln() { // @vuln
  final t = src('clos-to-sort-vuln'); // @source(clos-to-sort-vuln)
  final s = <String>[t, t];
  s.sort((a, b) {
    sinkCmd(a); // @sink(clos-to-sort-vuln)
    return a.compareTo(b);
  });
}

// vuln2: the elements are the arguments of the function given to reduce.
void vuln2() { // @vuln
  final t = src('clos-to-sort-vuln2'); // @source(clos-to-sort-vuln2)
  <String>[t, t].reduce((a, b) {
    sinkCmd(a); // @sink(clos-to-sort-vuln2)
    return b;
  });
}

void fixed() { // @fixed
  final t = cleanCmd(src('clos-to-sort-fixed'));
  final s = <String>[t, t];
  s.sort((a, b) {
    sinkCmd(a); // @sink(none)
    return a.compareTo(b);
  });
  s.reduce((a, b) {
    sinkCmd(a); // @sink(none)
    return b;
  });
}

void decoy() { // @decoy
  final t = src('clos-to-sort-decoy'); // @source(clos-to-sort-decoy)
  final s = <String>['x', 'y'];
  s.sort((a, b) {
    sinkCmd(a); // @sink(none) @nosink(clos-to-sort-decoy)
    return a.compareTo(b);
  });
  s.reduce((a, b) {
    sinkCmd(a); // @sink(none) @nosink(clos-to-sort-decoy)
    return b;
  });
  sinkSql(t); // @sink(clos-to-sort-decoy)
}
