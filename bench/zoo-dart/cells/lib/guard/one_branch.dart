// @cell(F/guard/one-branch)
// @shape: only one branch sanitizes (the case runs both branches)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/guard/one-branch/vuln', vuln);
  zooCase('F/guard/one-branch/fixed', fixed);
  zooCase('F/guard/one-branch/decoy', decoy);
}

void branch(String s, bool ok) {
  if (ok) {
    s = cleanCmd(s);
  }
  sinkCmd(s); // @sink(guard-one-branch-vuln)
}

void vuln() { // @vuln
  final v = src('guard-one-branch-vuln'); // @source(guard-one-branch-vuln)
  branch(v, true);
  branch(v, false);
}

void branchFixed(String s, bool ok) {
  if (ok) {
    s = cleanCmd(s);
  } else {
    s = cleanCmd(s);
  }
  sinkCmd(s); // @sink(none)
}

void fixed() { // @fixed
  final v = src('guard-one-branch-fixed');
  branchFixed(v, true);
  branchFixed(v, false);
}

void branchDecoy(String a, String b, bool ok) {
  if (ok) {
    a = cleanCmd(a);
  }
  sinkCmd(b); // @sink(none) @nosink(guard-one-branch-decoy)
  sinkSql(a); // @sink(guard-one-branch-decoy)
}

void decoy() { // @decoy
  final v = src('guard-one-branch-decoy'); // @source(guard-one-branch-decoy)
  branchDecoy(v, 'constant', true);
  branchDecoy(v, 'constant', false);
}
