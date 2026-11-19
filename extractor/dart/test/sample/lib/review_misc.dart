import 'lib.dart';

// What a review of the extractor asked for (REVIEW-2026-11-19): the rest.

// --- DART-5: a member of a dynamic value ------------------------------------

void dynamicMember() {
  final dynamic j = source(); // @source(rv-dynamic)
  sink(j.name); // @sink(rv-dynamic)
}

// --- DART-6: the initializer of a top level variable ------------------------

final String topLevel = source(); // @source(rv-top)

void readTopLevel() {
  sink(topLevel); // @sink(rv-top)
}

// --- DART-7: continue to a label of a switch case ---------------------------

void continueToCase(int k) {
  var x = 'safe';
  switch (k) {
    case 1:
      x = source(); // @source(rv-continue)
      continue last;
    case 2:
      x = 'other'; // what falling into the next case would run
    last:
    case 3:
      sink(x); // @sink(rv-continue)
  }
}

// --- DART-8: an assignment by pattern is a change of the variable -----------

(String, int) parsed() => (source(), 1); // @source(rv-pattern)

void patternAssignAfterClosure() {
  var name = 'safe';
  final cb = () => sink(name); // @sink(rv-pattern)
  (name, _) = parsed();
  cb();
}

// --- DART-9: the value of a future reaches the callback of then -------------

void futureThen() {
  final s = source(); // @source(rv-then)
  Future.value(s).then((v) {
    sink(v); // @sink(rv-then)
  });
}

// --- DART-10: an operator that the project defines is a call ---------------

class Bag {
  Bag(this.items);
  final List<String> items;
  void operator <<(String s) {
    sink(s); // @sink(rv-operator)
  }

  String operator [](int i) => items[i];
}

void userOperators() {
  final b = Bag([]);
  final s = source(); // @source(rv-operator)
  b << s;
}

void userIndexOperator() {
  final s = source(); // @source(rv-index-operator)
  final b = Bag([s]);
  sink(b[0]); // @sink(rv-index-operator)
}
