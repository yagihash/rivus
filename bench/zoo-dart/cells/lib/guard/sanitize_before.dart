// @cell(F/guard/sanitize-before)
// @shape: a value is sunk before and after a sanitizer (the decoy sanitizes a copy, and the other sink gets the original)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/guard/sanitize-before/vuln', vuln);
  zooCase('F/guard/sanitize-before/fixed', fixed);
  zooCase('F/guard/sanitize-before/decoy', decoy);
}

void vuln() { // @vuln
  var v = src('guard-sanitize-before-vuln'); // @source(guard-sanitize-before-vuln)
  sinkCmd(v); // @sink(guard-sanitize-before-vuln)
  v = cleanCmd(v);
  sinkCmd(v); // @sink(none)
}

void fixed() { // @fixed
  final v = cleanCmd(src('guard-sanitize-before-fixed'));
  sinkCmd(v); // @sink(none)
}

void decoy() { // @decoy
  final a = src('guard-sanitize-before-decoy'); // @source(guard-sanitize-before-decoy)
  final b = cleanCmd(a);
  sinkCmd(b); // @sink(none) @nosink(guard-sanitize-before-decoy)
  sinkSql(a); // @sink(guard-sanitize-before-decoy)
}
