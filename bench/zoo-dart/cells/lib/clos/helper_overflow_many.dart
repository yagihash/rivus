// @cell(F/clos/helper-overflow-many)
// @shape: thirty-four different closures are passed to one helper that calls its function argument (more candidates than the 32 that rivus keeps, ADR-050)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/helper-overflow-many/vuln', vuln);
  zooCase('F/clos/helper-overflow-many/vuln2', vuln2);
  zooCase('F/clos/helper-overflow-many/vuln3', vuln3);
  zooCase('F/clos/helper-overflow-many/vuln4', vuln4);
  zooCase('F/clos/helper-overflow-many/vuln5', vuln5);
  zooCase('F/clos/helper-overflow-many/vuln6', vuln6);
  zooCase('F/clos/helper-overflow-many/vuln7', vuln7);
  zooCase('F/clos/helper-overflow-many/vuln8', vuln8);
  zooCase('F/clos/helper-overflow-many/vuln9', vuln9);
  zooCase('F/clos/helper-overflow-many/vuln10', vuln10);
  zooCase('F/clos/helper-overflow-many/vuln11', vuln11);
  zooCase('F/clos/helper-overflow-many/vuln12', vuln12);
  zooCase('F/clos/helper-overflow-many/vuln13', vuln13);
  zooCase('F/clos/helper-overflow-many/vuln14', vuln14);
  zooCase('F/clos/helper-overflow-many/vuln15', vuln15);
  zooCase('F/clos/helper-overflow-many/vuln16', vuln16);
  zooCase('F/clos/helper-overflow-many/vuln17', vuln17);
  zooCase('F/clos/helper-overflow-many/vuln18', vuln18);
  zooCase('F/clos/helper-overflow-many/vuln19', vuln19);
  zooCase('F/clos/helper-overflow-many/vuln20', vuln20);
  zooCase('F/clos/helper-overflow-many/vuln21', vuln21);
  zooCase('F/clos/helper-overflow-many/vuln22', vuln22);
  zooCase('F/clos/helper-overflow-many/vuln23', vuln23);
  zooCase('F/clos/helper-overflow-many/vuln24', vuln24);
  zooCase('F/clos/helper-overflow-many/vuln25', vuln25);
  zooCase('F/clos/helper-overflow-many/vuln26', vuln26);
  zooCase('F/clos/helper-overflow-many/vuln27', vuln27);
  zooCase('F/clos/helper-overflow-many/vuln28', vuln28);
  zooCase('F/clos/helper-overflow-many/vuln29', vuln29);
  zooCase('F/clos/helper-overflow-many/vuln30', vuln30);
  zooCase('F/clos/helper-overflow-many/vuln31', vuln31);
  zooCase('F/clos/helper-overflow-many/vuln32', vuln32);
  zooCase('F/clos/helper-overflow-many/vuln33', vuln33);
  zooCase('F/clos/helper-overflow-many/vuln34', vuln34);
  zooCase('F/clos/helper-overflow-many/fixed', fixed);
  zooCase('F/clos/helper-overflow-many/decoy', decoy);
}

String transform(String s, String Function(String) f) => f(s);

void vuln() { // @vuln
  final a = src('clos-helper-overflow-many-vuln'); // @source(clos-helper-overflow-many-vuln) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln)
    return x + '1';
  });
}

void vuln2() { // @vuln
  final a = src('clos-helper-overflow-many-vuln2'); // @source(clos-helper-overflow-many-vuln2) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln2)
    return x + '2';
  });
}

void vuln3() { // @vuln
  final a = src('clos-helper-overflow-many-vuln3'); // @source(clos-helper-overflow-many-vuln3) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln3)
    return x + '3';
  });
}

void vuln4() { // @vuln
  final a = src('clos-helper-overflow-many-vuln4'); // @source(clos-helper-overflow-many-vuln4) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln4)
    return x + '4';
  });
}

void vuln5() { // @vuln
  final a = src('clos-helper-overflow-many-vuln5'); // @source(clos-helper-overflow-many-vuln5) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln5)
    return x + '5';
  });
}

void vuln6() { // @vuln
  final a = src('clos-helper-overflow-many-vuln6'); // @source(clos-helper-overflow-many-vuln6) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln6)
    return x + '6';
  });
}

void vuln7() { // @vuln
  final a = src('clos-helper-overflow-many-vuln7'); // @source(clos-helper-overflow-many-vuln7) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln7)
    return x + '7';
  });
}

void vuln8() { // @vuln
  final a = src('clos-helper-overflow-many-vuln8'); // @source(clos-helper-overflow-many-vuln8) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln8)
    return x + '8';
  });
}

void vuln9() { // @vuln
  final a = src('clos-helper-overflow-many-vuln9'); // @source(clos-helper-overflow-many-vuln9) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln9)
    return x + '9';
  });
}

void vuln10() { // @vuln
  final a = src('clos-helper-overflow-many-vuln10'); // @source(clos-helper-overflow-many-vuln10) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln10)
    return x + '10';
  });
}

void vuln11() { // @vuln
  final a = src('clos-helper-overflow-many-vuln11'); // @source(clos-helper-overflow-many-vuln11) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln11)
    return x + '11';
  });
}

void vuln12() { // @vuln
  final a = src('clos-helper-overflow-many-vuln12'); // @source(clos-helper-overflow-many-vuln12) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln12)
    return x + '12';
  });
}

void vuln13() { // @vuln
  final a = src('clos-helper-overflow-many-vuln13'); // @source(clos-helper-overflow-many-vuln13) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln13)
    return x + '13';
  });
}

void vuln14() { // @vuln
  final a = src('clos-helper-overflow-many-vuln14'); // @source(clos-helper-overflow-many-vuln14) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln14)
    return x + '14';
  });
}

void vuln15() { // @vuln
  final a = src('clos-helper-overflow-many-vuln15'); // @source(clos-helper-overflow-many-vuln15) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln15)
    return x + '15';
  });
}

void vuln16() { // @vuln
  final a = src('clos-helper-overflow-many-vuln16'); // @source(clos-helper-overflow-many-vuln16) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln16)
    return x + '16';
  });
}

void vuln17() { // @vuln
  final a = src('clos-helper-overflow-many-vuln17'); // @source(clos-helper-overflow-many-vuln17) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln17)
    return x + '17';
  });
}

void vuln18() { // @vuln
  final a = src('clos-helper-overflow-many-vuln18'); // @source(clos-helper-overflow-many-vuln18) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln18)
    return x + '18';
  });
}

void vuln19() { // @vuln
  final a = src('clos-helper-overflow-many-vuln19'); // @source(clos-helper-overflow-many-vuln19) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln19)
    return x + '19';
  });
}

void vuln20() { // @vuln
  final a = src('clos-helper-overflow-many-vuln20'); // @source(clos-helper-overflow-many-vuln20) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln20)
    return x + '20';
  });
}

void vuln21() { // @vuln
  final a = src('clos-helper-overflow-many-vuln21'); // @source(clos-helper-overflow-many-vuln21) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln21)
    return x + '21';
  });
}

void vuln22() { // @vuln
  final a = src('clos-helper-overflow-many-vuln22'); // @source(clos-helper-overflow-many-vuln22) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln22)
    return x + '22';
  });
}

void vuln23() { // @vuln
  final a = src('clos-helper-overflow-many-vuln23'); // @source(clos-helper-overflow-many-vuln23) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln23)
    return x + '23';
  });
}

void vuln24() { // @vuln
  final a = src('clos-helper-overflow-many-vuln24'); // @source(clos-helper-overflow-many-vuln24) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln24)
    return x + '24';
  });
}

void vuln25() { // @vuln
  final a = src('clos-helper-overflow-many-vuln25'); // @source(clos-helper-overflow-many-vuln25) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln25)
    return x + '25';
  });
}

void vuln26() { // @vuln
  final a = src('clos-helper-overflow-many-vuln26'); // @source(clos-helper-overflow-many-vuln26) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln26)
    return x + '26';
  });
}

void vuln27() { // @vuln
  final a = src('clos-helper-overflow-many-vuln27'); // @source(clos-helper-overflow-many-vuln27) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln27)
    return x + '27';
  });
}

void vuln28() { // @vuln
  final a = src('clos-helper-overflow-many-vuln28'); // @source(clos-helper-overflow-many-vuln28) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln28)
    return x + '28';
  });
}

void vuln29() { // @vuln
  final a = src('clos-helper-overflow-many-vuln29'); // @source(clos-helper-overflow-many-vuln29) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln29)
    return x + '29';
  });
}

void vuln30() { // @vuln
  final a = src('clos-helper-overflow-many-vuln30'); // @source(clos-helper-overflow-many-vuln30) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln30)
    return x + '30';
  });
}

void vuln31() { // @vuln
  final a = src('clos-helper-overflow-many-vuln31'); // @source(clos-helper-overflow-many-vuln31) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln31)
    return x + '31';
  });
}

void vuln32() { // @vuln
  final a = src('clos-helper-overflow-many-vuln32'); // @source(clos-helper-overflow-many-vuln32) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln32)
    return x + '32';
  });
}

void vuln33() { // @vuln
  final a = src('clos-helper-overflow-many-vuln33'); // @source(clos-helper-overflow-many-vuln33) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln33)
    return x + '33';
  });
}

void vuln34() { // @vuln
  final a = src('clos-helper-overflow-many-vuln34'); // @source(clos-helper-overflow-many-vuln34) @miss(depth_limit) @ref(#77)
  transform(a, (x) {
    sinkCmd(x); // @sink(clos-helper-overflow-many-vuln34)
    return x + '34';
  });
}

void fixed() { // @fixed
  final a = cleanCmd(src('clos-helper-overflow-many-fixed'));
  transform(a, (x) {
    sinkCmd(x); // @sink(none)
    return x + 'f';
  });
}

void decoy() { // @decoy
  final a = src('clos-helper-overflow-many-decoy'); // @source(clos-helper-overflow-many-decoy)
  transform('constant', (x) {
    sinkCmd(x); // @sink(none) @nosink(clos-helper-overflow-many-decoy)
    return x + 'd';
  });
  sinkSql(a); // @sink(clos-helper-overflow-many-decoy)
}
