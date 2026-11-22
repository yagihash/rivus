import 'lib.dart';

// A helper that thirty-three different functions are given to: more than the thirty-two
// candidates rivus keeps for a call of a function value (ADR-050; 8 until 2026-11-22). The functions
// are known; rivus leaves them out, and says so (depth_limit, candidate_limit).

String callIt(String s, String Function(String) f) => f(s);

String f1(String s) => s;
String f2(String s) => s;
String f3(String s) => s;
String f4(String s) => s;
String f5(String s) => s;
String f6(String s) => s;
String f7(String s) => s;
String f8(String s) => s;
String f9(String s) => s;
String f10(String s) => s;
String f11(String s) => s;
String f12(String s) => s;
String f13(String s) => s;
String f14(String s) => s;
String f15(String s) => s;
String f16(String s) => s;
String f17(String s) => s;
String f18(String s) => s;
String f19(String s) => s;
String f20(String s) => s;
String f21(String s) => s;
String f22(String s) => s;
String f23(String s) => s;
String f24(String s) => s;
String f25(String s) => s;
String f26(String s) => s;
String f27(String s) => s;
String f28(String s) => s;
String f29(String s) => s;
String f30(String s) => s;
String f31(String s) => s;
String f32(String s) => s;

void limit_many() {
  final a = source(); // @source(limit-many) @miss(silent)
  callIt(a, f1);
  callIt(a, f2);
  callIt(a, f3);
  callIt(a, f4);
  callIt(a, f5);
  callIt(a, f6);
  callIt(a, f7);
  callIt(a, f8);
  callIt(a, f9);
  callIt(a, f10);
  callIt(a, f11);
  callIt(a, f12);
  callIt(a, f13);
  callIt(a, f14);
  callIt(a, f15);
  callIt(a, f16);
  callIt(a, f17);
  callIt(a, f18);
  callIt(a, f19);
  callIt(a, f20);
  callIt(a, f21);
  callIt(a, f22);
  callIt(a, f23);
  callIt(a, f24);
  callIt(a, f25);
  callIt(a, f26);
  callIt(a, f27);
  callIt(a, f28);
  callIt(a, f29);
  callIt(a, f30);
  callIt(a, f31);
  callIt(a, f32);
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
