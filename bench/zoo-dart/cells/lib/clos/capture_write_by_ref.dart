// @cell(F/clos/capture-write-by-ref)
// @shape: a closure captures a variable (every capture of Dart is the variable itself): the variable is written after the closure is made, or the closure writes it and the outside reads it; also a loop variable, and the closure given to a helper of the project or to a function of the core library (forEach)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/capture-write-by-ref/vuln', vuln);
  zooCase('F/clos/capture-write-by-ref/vuln2', vuln2);
  zooCase('F/clos/capture-write-by-ref/vuln3', vuln3);
  zooCase('F/clos/capture-write-by-ref/vuln4', vuln4);
  zooCase('F/clos/capture-write-by-ref/vuln5', vuln5);
  zooCase('F/clos/capture-write-by-ref/vuln6', vuln6);
  zooCase('F/clos/capture-write-by-ref/vuln7', vuln7);
  zooCase('F/clos/capture-write-by-ref/fixed', fixed);
  zooCase('F/clos/capture-write-by-ref/decoy', decoy);
}

void run(void Function() f) => f();

// vuln: written after the closure is made, then the closure is called.
void vuln() { // @vuln
  var x = '';
  void f() {
    sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln)
  }
  x = src('clos-capture-write-by-ref-vuln'); // @source(clos-capture-write-by-ref-vuln)
  f();
}

// vuln2: the closure writes the variable; the outside reads it.
void vuln2() { // @vuln
  var x = '';
  void f() {
    x = src('clos-capture-write-by-ref-vuln2'); // @source(clos-capture-write-by-ref-vuln2)
  }
  f();
  sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln2)
}

// vuln3: the variable of a for-in (one variable for each iteration); the closures are called later.
void vuln3() { // @vuln
  final fs = <void Function()>[];
  for (final s in [src('clos-capture-write-by-ref-vuln3')]) { // @source(clos-capture-write-by-ref-vuln3)
    fs.add(() {
      sinkCmd(s); // @sink(clos-capture-write-by-ref-vuln3)
    });
  }
  for (final f in fs) {
    f();
  }
}

// vuln4: written after the closure is made; a helper of the project calls it.
void vuln4() { // @vuln
  var x = '';
  void f() {
    sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln4)
  }
  x = src('clos-capture-write-by-ref-vuln4'); // @source(clos-capture-write-by-ref-vuln4)
  run(f);
}

// vuln5: written after the closure is made; forEach of the core library calls it.
void vuln5() { // @vuln
  var x = '';
  void f(int _) {
    sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln5)
  }
  x = src('clos-capture-write-by-ref-vuln5'); // @source(clos-capture-write-by-ref-vuln5)
  [1].forEach(f);
}

// vuln6: the closure writes, a helper of the project calls it; the outside reads.
void vuln6() { // @vuln
  var x = '';
  run(() {
    x = src('clos-capture-write-by-ref-vuln6'); // @source(clos-capture-write-by-ref-vuln6)
  });
  sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln6)
}

// vuln7: the closure writes, forEach calls it; the outside reads.
void vuln7() { // @vuln
  var x = '';
  [1].forEach((_) {
    x = src('clos-capture-write-by-ref-vuln7'); // @source(clos-capture-write-by-ref-vuln7)
  });
  sinkCmd(x); // @sink(clos-capture-write-by-ref-vuln7)
}

void fixed() { // @fixed
  var x = '';
  void f() {
    sinkCmd(x); // @sink(none)
  }
  x = cleanCmd(src('clos-capture-write-by-ref-fixed'));
  f();
  var y = '';
  run(() {
    y = cleanCmd(src('clos-capture-write-by-ref-fixed'));
  });
  sinkCmd(y); // @sink(none)
}

void decoy() { // @decoy
  var x = '';
  const y = 'constant';
  void set() {
    x = src('clos-capture-write-by-ref-decoy'); // @source(clos-capture-write-by-ref-decoy)
  }
  void read() {
    sinkSql(x); // @sink(clos-capture-write-by-ref-decoy)
  }
  void quiet() {
    sinkCmd(y); // @sink(none) @nosink(clos-capture-write-by-ref-decoy)
  }
  run(set);
  run(read);
  run(quiet);
}
