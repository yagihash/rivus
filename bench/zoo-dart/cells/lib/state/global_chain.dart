// @cell(F/state/global-chain)
// @shape: a value goes through a chain of top-level variables, one function per stage (3, 13 and 30 stages)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/global-chain/vuln', vuln);
  zooCase('F/state/global-chain/vuln13', vuln13);
  zooCase('F/state/global-chain/vuln30', vuln30);
  zooCase('F/state/global-chain/fixed', fixed);
  zooCase('F/state/global-chain/decoy', decoy);
}


String c3g0 = '';
String c3g1 = '';
String c3g2 = '';
String c3g3 = '';

void c3s1() {
  c3g1 = c3g0;
}

void c3s2() {
  c3g2 = c3g1;
}

void c3s3() {
  c3g3 = c3g2;
}


void vuln() { // @vuln
  c3g0 = src('state-global-chain-vuln'); // @source(state-global-chain-vuln)
  c3s1();
  c3s2();
  c3s3();
  sinkCmd(c3g3); // @sink(state-global-chain-vuln)
}


String c13g0 = '';
String c13g1 = '';
String c13g2 = '';
String c13g3 = '';
String c13g4 = '';
String c13g5 = '';
String c13g6 = '';
String c13g7 = '';
String c13g8 = '';
String c13g9 = '';
String c13g10 = '';
String c13g11 = '';
String c13g12 = '';
String c13g13 = '';

void c13s1() {
  c13g1 = c13g0;
}

void c13s2() {
  c13g2 = c13g1;
}

void c13s3() {
  c13g3 = c13g2;
}

void c13s4() {
  c13g4 = c13g3;
}

void c13s5() {
  c13g5 = c13g4;
}

void c13s6() {
  c13g6 = c13g5;
}

void c13s7() {
  c13g7 = c13g6;
}

void c13s8() {
  c13g8 = c13g7;
}

void c13s9() {
  c13g9 = c13g8;
}

void c13s10() {
  c13g10 = c13g9;
}

void c13s11() {
  c13g11 = c13g10;
}

void c13s12() {
  c13g12 = c13g11;
}

void c13s13() {
  c13g13 = c13g12;
}


void vuln13() { // @vuln
  c13g0 = src('state-global-chain-vuln13'); // @source(state-global-chain-vuln13) @miss(recursion_approximation) @ref(PASS-LIMIT)
  c13s1();
  c13s2();
  c13s3();
  c13s4();
  c13s5();
  c13s6();
  c13s7();
  c13s8();
  c13s9();
  c13s10();
  c13s11();
  c13s12();
  c13s13();
  sinkCmd(c13g13); // @sink(state-global-chain-vuln13)
}


String c30g0 = '';
String c30g1 = '';
String c30g2 = '';
String c30g3 = '';
String c30g4 = '';
String c30g5 = '';
String c30g6 = '';
String c30g7 = '';
String c30g8 = '';
String c30g9 = '';
String c30g10 = '';
String c30g11 = '';
String c30g12 = '';
String c30g13 = '';
String c30g14 = '';
String c30g15 = '';
String c30g16 = '';
String c30g17 = '';
String c30g18 = '';
String c30g19 = '';
String c30g20 = '';
String c30g21 = '';
String c30g22 = '';
String c30g23 = '';
String c30g24 = '';
String c30g25 = '';
String c30g26 = '';
String c30g27 = '';
String c30g28 = '';
String c30g29 = '';
String c30g30 = '';

void c30s1() {
  c30g1 = c30g0;
}

void c30s2() {
  c30g2 = c30g1;
}

void c30s3() {
  c30g3 = c30g2;
}

void c30s4() {
  c30g4 = c30g3;
}

void c30s5() {
  c30g5 = c30g4;
}

void c30s6() {
  c30g6 = c30g5;
}

void c30s7() {
  c30g7 = c30g6;
}

void c30s8() {
  c30g8 = c30g7;
}

void c30s9() {
  c30g9 = c30g8;
}

void c30s10() {
  c30g10 = c30g9;
}

void c30s11() {
  c30g11 = c30g10;
}

void c30s12() {
  c30g12 = c30g11;
}

void c30s13() {
  c30g13 = c30g12;
}

void c30s14() {
  c30g14 = c30g13;
}

void c30s15() {
  c30g15 = c30g14;
}

void c30s16() {
  c30g16 = c30g15;
}

void c30s17() {
  c30g17 = c30g16;
}

void c30s18() {
  c30g18 = c30g17;
}

void c30s19() {
  c30g19 = c30g18;
}

void c30s20() {
  c30g20 = c30g19;
}

void c30s21() {
  c30g21 = c30g20;
}

void c30s22() {
  c30g22 = c30g21;
}

void c30s23() {
  c30g23 = c30g22;
}

void c30s24() {
  c30g24 = c30g23;
}

void c30s25() {
  c30g25 = c30g24;
}

void c30s26() {
  c30g26 = c30g25;
}

void c30s27() {
  c30g27 = c30g26;
}

void c30s28() {
  c30g28 = c30g27;
}

void c30s29() {
  c30g29 = c30g28;
}

void c30s30() {
  c30g30 = c30g29;
}


void vuln30() { // @vuln
  c30g0 = src('state-global-chain-vuln30'); // @source(state-global-chain-vuln30) @miss(recursion_approximation) @ref(PASS-LIMIT)
  c30s1();
  c30s2();
  c30s3();
  c30s4();
  c30s5();
  c30s6();
  c30s7();
  c30s8();
  c30s9();
  c30s10();
  c30s11();
  c30s12();
  c30s13();
  c30s14();
  c30s15();
  c30s16();
  c30s17();
  c30s18();
  c30s19();
  c30s20();
  c30s21();
  c30s22();
  c30s23();
  c30s24();
  c30s25();
  c30s26();
  c30s27();
  c30s28();
  c30s29();
  c30s30();
  sinkCmd(c30g30); // @sink(state-global-chain-vuln30)
}


String f3g0 = '';
String f3g1 = '';
String f3g2 = '';
String f3g3 = '';

void f3s1() {
  f3g1 = f3g0;
}

void f3s2() {
  f3g2 = f3g1;
}

void f3s3() {
  f3g3 = f3g2;
}


void fixed() { // @fixed
  f3g0 = cleanCmd(src('state-global-chain-fixed'));
  f3s1();
  f3s2();
  f3s3();
  sinkCmd(f3g3); // @sink(none)
}


String d3g0 = '';
String d3g1 = '';
String d3g2 = '';
String d3g3 = '';

void d3s1() {
  d3g1 = d3g0;
}

void d3s2() {
  d3g2 = d3g1;
}

void d3s3() {
  d3g3 = d3g2;
}



String e3g0 = '';
String e3g1 = '';
String e3g2 = '';
String e3g3 = '';

void e3s1() {
  e3g1 = e3g0;
}

void e3s2() {
  e3g2 = e3g1;
}

void e3s3() {
  e3g3 = e3g2;
}


void decoy() { // @decoy
  d3g0 = src('state-global-chain-decoy'); // @source(state-global-chain-decoy)
  e3g0 = 'constant';
  d3s1();
  d3s2();
  d3s3();
  e3s1();
  e3s2();
  e3s3();
  sinkCmd(e3g3); // @sink(none) @nosink(state-global-chain-decoy)
  sinkSql(d3g3); // @sink(state-global-chain-decoy)
}
