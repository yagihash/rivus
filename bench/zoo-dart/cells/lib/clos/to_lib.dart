// @cell(F/clos/to-lib)
// @shape: a closure that captures a value is given to a function of the core library that calls it (forEach, map, Future.then)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/to-lib/vuln', vuln);
  zooCase('F/clos/to-lib/vuln2', vuln2);
  zooCase('F/clos/to-lib/vuln3', vuln3);
  zooCase('F/clos/to-lib/vuln4', vuln4);
  zooCase('F/clos/to-lib/fixed', fixed);
  zooCase('F/clos/to-lib/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('clos-to-lib-vuln'); // @source(clos-to-lib-vuln)
  [1].forEach((_) {
    sinkCmd(v); // @sink(clos-to-lib-vuln)
  });
}

// vuln2: the result of the closure comes back through the library.
void vuln2() { // @vuln
  final v = src('clos-to-lib-vuln2'); // @source(clos-to-lib-vuln2)
  sinkCmd([1].map((_) => v).first); // @sink(clos-to-lib-vuln2)
}

// vuln3: the same with a Future.
Future<void> vuln3() async { // @vuln
  final v = src('clos-to-lib-vuln3'); // @source(clos-to-lib-vuln3)
  final r = await Future.value(1).then((_) => v);
  sinkCmd(r); // @sink(clos-to-lib-vuln3)
}

// vuln4: the value of the stream is what the callback of asyncMap is called with.
Future<void> vuln4() async { // @vuln
  final v = src('clos-to-lib-vuln4'); // @source(clos-to-lib-vuln4)
  await Stream<String>.value(v).asyncMap((e) async {
    sinkCmd(e); // @sink(clos-to-lib-vuln4)
    return e;
  }).drain<void>();
}

void fixed() { // @fixed
  final v = cleanCmd(src('clos-to-lib-fixed'));
  [1].forEach((_) {
    sinkCmd(v); // @sink(none)
  });
}

void decoy() { // @decoy
  final v = src('clos-to-lib-decoy'); // @source(clos-to-lib-decoy)
  const k = 'constant';
  [1].forEach((_) {
    sinkCmd(k); // @sink(none) @nosink(clos-to-lib-decoy)
  });
  sinkSql([1].map((_) => v).first); // @sink(clos-to-lib-decoy)
}
