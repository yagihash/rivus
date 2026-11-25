// @cell(F/clos/to-lib-iter)
// @shape: a function that is given to Iterable.singleWhere or Iterable.expand is called with the elements of the receiver: what the iterable holds reaches the sink in the function, and what the function of expand returns comes back in the result
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-lib-iter/vuln', vuln);
  zooCase('F/clos/to-lib-iter/vuln2', vuln2);
  zooCase('F/clos/to-lib-iter/vuln3', vuln3);
  zooCase('F/clos/to-lib-iter/fixed', fixed);
  zooCase('F/clos/to-lib-iter/decoy', decoy);
}

void vuln() { // @vuln
  final t = src('clos-to-lib-iter-vuln'); // @source(clos-to-lib-iter-vuln)
  <String>[t].singleWhere((x) {
    sinkCmd(x); // @sink(clos-to-lib-iter-vuln)
    return true;
  });
}

// vuln2: the element is the argument of the function given to expand.
void vuln2() { // @vuln
  final t = src('clos-to-lib-iter-vuln2'); // @source(clos-to-lib-iter-vuln2)
  <String>[t].expand((x) {
    sinkCmd(x); // @sink(clos-to-lib-iter-vuln2)
    return <String>[];
  }).toList();
}

// vuln3: what the function of expand returns is the elements of the result.
void vuln3() { // @vuln
  final t = src('clos-to-lib-iter-vuln3'); // @source(clos-to-lib-iter-vuln3)
  sinkCmd(<int>[1].expand((_) => <String>[t]).first); // @sink(clos-to-lib-iter-vuln3)
}

void fixed() { // @fixed
  final t = cleanCmd(src('clos-to-lib-iter-fixed'));
  <String>[t].singleWhere((x) {
    sinkCmd(x); // @sink(none)
    return true;
  });
  <String>[t].expand((x) {
    sinkCmd(x); // @sink(none)
    return <String>[];
  }).toList();
  sinkCmd(<int>[1].expand((_) => <String>[t]).first); // @sink(none)
}

void decoy() { // @decoy
  final t = src('clos-to-lib-iter-decoy'); // @source(clos-to-lib-iter-decoy)
  const k = 'constant';
  <String>['x'].singleWhere((x) {
    sinkCmd(k); // @sink(none) @nosink(clos-to-lib-iter-decoy)
    return true;
  });
  <String>['x'].expand((x) {
    sinkCmd(k); // @sink(none) @nosink(clos-to-lib-iter-decoy)
    return <String>[];
  }).toList();
  sinkSql(t); // @sink(clos-to-lib-iter-decoy)
}
