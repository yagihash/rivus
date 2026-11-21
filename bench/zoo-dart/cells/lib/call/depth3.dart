// @cell(F/call/depth3)
// @shape: a value goes down three calls to a sink (the decoy passes two values down, and only the constant is sunk where the quiet sink is)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/call/depth3/vuln', vuln);
  zooCase('F/call/depth3/fixed', fixed);
  zooCase('F/call/depth3/decoy', decoy);
}

void f1(String s) => f2(s);
void f2(String s) => f3(s);
void f3(String s) {
  sinkCmd(s); // @sink(call-depth3-vuln)
}

void vuln() { // @vuln
  final v = src('call-depth3-vuln'); // @source(call-depth3-vuln)
  f1(v);
}

void g1(String s) => g2(s);
void g2(String s) => g3(s);
void g3(String s) {
  sinkCmd(s); // @sink(none)
}

void fixed() { // @fixed
  final v = cleanCmd(src('call-depth3-fixed'));
  g1(v);
}

void h1(String a, String b) => h2(a, b);
void h2(String a, String b) => h3(a, b);
void h3(String a, String b) {
  sinkCmd(b); // @sink(none) @nosink(call-depth3-decoy)
  sinkSql(a); // @sink(call-depth3-decoy)
}

void decoy() { // @decoy
  final t = src('call-depth3-decoy'); // @source(call-depth3-decoy)
  h1(t, 'constant');
}
