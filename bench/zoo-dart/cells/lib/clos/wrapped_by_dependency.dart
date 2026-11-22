// @cell(F/clos/wrapped-by-dependency)
// @shape: a function of the project is given to a dependency (zoo_dep.wrapFn), which returns a closure that calls it; the project calls what it got back
// (the closure of the dependency holds the function that it was given in a captured cell, and the call of the value that comes back has no candidate: it is not silent, the call says unresolved_callee and the findings that go through it carry it.)
import 'package:zoo_dep/zoo_dep.dart' as dep;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/wrapped-by-dependency/vuln', vuln);
  zooCase('F/clos/wrapped-by-dependency/fixed', fixed);
  zooCase('F/clos/wrapped-by-dependency/decoy', decoy);
}

void vuln() { // @vuln
  final a = src('clos-wrapped-by-dependency-vuln'); // @source(clos-wrapped-by-dependency-vuln)
  final g = dep.wrapFn((s) => s);
  sinkCmd(g(a)); // @sink(clos-wrapped-by-dependency-vuln)
}

void fixed() { // @fixed
  final a = src('clos-wrapped-by-dependency-fixed');
  final g = dep.wrapFn((s) => cleanCmd(s));
  sinkCmd(g(a)); // @sink(none) @fp @ref(XTALK)
}

void decoy() { // @decoy
  final a = src('clos-wrapped-by-dependency-decoy'); // @source(clos-wrapped-by-dependency-decoy)
  final quiet = dep.wrapFn((s) => 'constant');
  final loud = dep.wrapFn((s) => s);
  sinkCmd(quiet(a)); // @sink(none) @fp @nosink(clos-wrapped-by-dependency-decoy) @fp @ref(XTALK)
  sinkSql(loud(a)); // @sink(clos-wrapped-by-dependency-decoy)
}
