// @cell(F/clos/local-recursion)
// @shape: a local function calls itself by its name, and the value reaches the sink only through the recursive call
// @na(go: a Go function declared inside another cannot name itself; the nearest form, a var that holds a closure that calls itself, is a call of a value)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/local-recursion/vuln', vuln);
  zooCase('F/clos/local-recursion/fixed', fixed);
  zooCase('F/clos/local-recursion/decoy', decoy);
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
