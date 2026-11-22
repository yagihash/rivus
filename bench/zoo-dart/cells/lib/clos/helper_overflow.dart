// @cell(F/clos/helper-overflow)
// @shape: ten different closures are passed to one helper that calls its function argument (more than eight candidates)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/helper-overflow/vuln', vuln);
  zooCase('F/clos/helper-overflow/vuln2', vuln2);
  zooCase('F/clos/helper-overflow/vuln3', vuln3);
  zooCase('F/clos/helper-overflow/vuln4', vuln4);
  zooCase('F/clos/helper-overflow/vuln5', vuln5);
  zooCase('F/clos/helper-overflow/vuln6', vuln6);
  zooCase('F/clos/helper-overflow/vuln7', vuln7);
  zooCase('F/clos/helper-overflow/vuln8', vuln8);
  zooCase('F/clos/helper-overflow/vuln9', vuln9);
  zooCase('F/clos/helper-overflow/vuln10', vuln10);
  zooCase('F/clos/helper-overflow/fixed', fixed);
  zooCase('F/clos/helper-overflow/decoy', decoy);
}

String transform(String s, String Function(String) f) => f(s);

void vuln() { // @vuln
  final a = src('clos-helper-overflow-vuln'); // @source(clos-helper-overflow-vuln)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln)
    return x + '1';
  });
}

void vuln2() { // @vuln
  final a = src('clos-helper-overflow-vuln2'); // @source(clos-helper-overflow-vuln2)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln2)
    return x + '2';
  });
}

void vuln3() { // @vuln
  final a = src('clos-helper-overflow-vuln3'); // @source(clos-helper-overflow-vuln3)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln3)
    return x + '3';
  });
}

void vuln4() { // @vuln
  final a = src('clos-helper-overflow-vuln4'); // @source(clos-helper-overflow-vuln4)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln4)
    return x + '4';
  });
}

void vuln5() { // @vuln
  final a = src('clos-helper-overflow-vuln5'); // @source(clos-helper-overflow-vuln5)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln5)
    return x + '5';
  });
}

void vuln6() { // @vuln
  final a = src('clos-helper-overflow-vuln6'); // @source(clos-helper-overflow-vuln6)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln6)
    return x + '6';
  });
}

void vuln7() { // @vuln
  final a = src('clos-helper-overflow-vuln7'); // @source(clos-helper-overflow-vuln7)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln7)
    return x + '7';
  });
}

void vuln8() { // @vuln
  final a = src('clos-helper-overflow-vuln8'); // @source(clos-helper-overflow-vuln8)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln8)
    return x + '8';
  });
}

void vuln9() { // @vuln
  final a = src('clos-helper-overflow-vuln9'); // @source(clos-helper-overflow-vuln9)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln9)
    return x + '9';
  });
}

void vuln10() { // @vuln
  final a = src('clos-helper-overflow-vuln10'); // @source(clos-helper-overflow-vuln10)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-vuln10)
    return x + '10';
  });
}

void fixed() { // @fixed
  final a = cleanCmd(src('clos-helper-overflow-fixed'));
  transform(a, (x) {
    sinkCmd(x); // @sink(none)
    return x + 'f';
  });
}

void decoy() { // @decoy
  final a = src('clos-helper-overflow-decoy'); // @source(clos-helper-overflow-decoy)
  transform('constant', (x) {
    sinkCmd(x); // @sink(none) @nosink(clos-helper-overflow-decoy)
    return x + 'd';
  });
  sinkSql(a); // @sink(clos-helper-overflow-decoy)
}
