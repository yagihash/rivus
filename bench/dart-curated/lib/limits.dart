import 'lib.dart';

// A helper that nine different closures are given to: more than the eight
// candidates rivus keeps for a call of a function value (ADR-050). The functions
// are known; rivus leaves them out, and says so (depth_limit, candidate_limit).

String callIt(String s, String Function(String) f) => f(s);

String one(String s) => s;
String two(String s) => s;
String three(String s) => s;
String four(String s) => s;
String five(String s) => s;
String six(String s) => s;
String seven(String s) => s;
String eight(String s) => s;

void limit_many() {
  final a = source(); // @source(limit-many) @miss(silent)
  callIt(a, one);
  callIt(a, two);
  callIt(a, three);
  callIt(a, four);
  callIt(a, five);
  callIt(a, six);
  callIt(a, seven);
  callIt(a, eight);
  callIt(a, (x) {
    sink(x); // @sink(limit-many)
    return x;
  });
}

// The closure is passed down five helpers, farther than the three calls up that are followed:
// the function is there, and rivus did not follow the callers far enough to see it
// (depth_limit, caller_depth_limit).
String w1(String s, String Function(String) f) => w2(s, f);
String w2(String s, String Function(String) f) => w3(s, f);
String w3(String s, String Function(String) f) => w4(s, f);
String w4(String s, String Function(String) f) => w5(s, f);
String w5(String s, String Function(String) f) => f(s);

void limit_deep() {
  final a = source(); // @source(limit-deep) @miss(silent)
  w1(a, (x) {
    sink(x); // @sink(limit-deep)
    return x;
  });
}
