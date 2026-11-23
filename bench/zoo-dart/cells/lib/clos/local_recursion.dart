// @cell(F/clos/local-recursion)
// @shape: a local function calls itself by its name, and the value reaches the sink only through the recursive call
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/local-recursion/vuln', vuln);
  zooCase('F/clos/local-recursion/fixed', fixed);
  zooCase('F/clos/local-recursion/decoy', decoy);
  zooCase('F/clos/local-recursion/vuln2', nested);
  zooCase('F/clos/local-recursion/vuln3', mutual);
}

void vuln() { // @vuln
  // the two arguments change places in each call: the first one reaches the sink in the second call
  void go(String x, String y, int n) {
    if (n == 0) {
      sinkCmd(y); // @sink(clos-local-recursion-vuln)
      return;
    }
    go(y, x, n - 1);
  }

  go(src('clos-local-recursion-vuln'), 'constant', 1); // @source(clos-local-recursion-vuln)
}

void fixed() { // @fixed
  void go(String x, String y, int n) {
    if (n == 0) {
      sinkCmd(y); // @sink(none)
      return;
    }
    go(y, x, n - 1);
  }

  go(cleanCmd(src('clos-local-recursion-fixed')), 'constant', 1);
}

void decoy() { // @decoy
  void go(String a, String b, int n) {
    if (n == 0) {
      sinkCmd(b); // @sink(none) @nosink(clos-local-recursion-decoy)
      sinkSql(a); // @sink(clos-local-recursion-decoy)
      return;
    }
    go(a, b, n - 1);
  }

  go(src('clos-local-recursion-decoy'), 'constant', 1); // @source(clos-local-recursion-decoy)
}

void nested() { // @vuln
  // a closure inside the local function calls the local function by its name
  void go(String x, String y, int n) {
    void again() {
      go(y, x, n - 1);
    }

    if (n == 0) {
      sinkCmd(y); // @sink(clos-local-recursion-nested)
      return;
    }
    again();
  }

  go(src('clos-local-recursion-nested'), 'constant', 1); // @source(clos-local-recursion-nested)
}

void mutual() { // @vuln
  // two local functions call each other
  late void Function(String, String, int) odd;
  void even(String x, String y, int n) {
    if (n == 0) {
      sinkCmd(y); // @sink(clos-local-recursion-mutual)
      return;
    }
    odd(y, x, n - 1);
  }

  odd = (String x, String y, int n) => even(x, y, n);
  even(src('clos-local-recursion-mutual'), 'constant', 1); // @source(clos-local-recursion-mutual) @miss(unresolved_callee) @ref(#196)
}
