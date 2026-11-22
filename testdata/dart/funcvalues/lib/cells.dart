// Calls of function values with no candidate: the function is held in a field of the object that the caller made, and
// the caller of `run` is not the one that made the closure. The analysis of the function values carries the closure
// there (docs/DESIGN-callouts.md, 5.6). Each cell is a pair of a source and a sink.
import 'mark.dart';

class Holder {
  final String Function(String) cb;
  Holder(this.cb);
}

String run(Holder h, String s) => h.cb(s);

// positive: the closure reaches the call in `run`, and passes what it is given to the sink
void pos() {
  final h = Holder((s) {
    sink(s); // @sink(pos)
    return s;
  });
  run(h, src()); // @source(pos)
}

// positive, the callback of a callback: the closure is called by a function that is called through a function value
String run2(Holder h, String s) => h.cb(s);

void pos2() {
  final inner = Holder((s) {
    sink(s); // @sink(pos2)
    return s;
  });
  final outer = Holder((s) => run2(inner, s));
  run2(outer, src()); // @source(pos2)
}

// negative: no closure reaches `runNone`. The decoy has the type of the field, and it is a closure of this code, but
// it is only called here
String runNone(Holder h, String s) => h.cb(s);

void negEntry(Holder unknown) {
  runNone(unknown, src()); // @source(none)
}

void negDecoy() {
  final decoy = (String s) {
    sink(s); // @sink(none)
    return s;
  };
  decoy('constant');
}

// negative: the closure that the call would get is made by a function of a dependency (the fixture marks `depMake` as
// one): it is not a seed, so the call keeps no candidate
String runDep(Holder h, String s) => h.cb(s);

Holder depMake() => Holder((s) {
      sink(s); // @sink(none)
      return s;
    });

void negDep() {
  runDep(depMake(), src()); // @source(none)
}

// negative: a closure is given to a real sink; the function value must not be a finding of its own
void exec(Object o) {}

void negExec() {
  exec((String s) => s);
}

// the closure is given to a function that the model says is pure: the function value goes nowhere (dropped_at_model)
void keep(Object o) {}

void negPure() {
  keep((String s) => s);
}
