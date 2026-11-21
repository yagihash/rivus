// @cell(F/conv/struct-field-boxed)
// @shape: a statement is given to the sink as an Object (the static type is lost), and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/struct-field-boxed/vuln', vuln);
  zooCase('F/conv/struct-field-boxed/fixed', fixed);
  zooCase('F/conv/struct-field-boxed/decoy', decoy);
}

void vuln() { // @vuln
  final req = Stmt();
  req.sql = src('conv-struct-field-boxed-vuln'); // @source(conv-struct-field-boxed-vuln)
  final Object v = req;
  sinkStmtAny(v); // @sink(conv-struct-field-boxed-vuln)
}

void fixed() { // @fixed
  final req = Stmt();
  req.sql = cleanSql(src('conv-struct-field-boxed-fixed'));
  final Object v = req;
  sinkStmtAny(v); // @sink(none)
}

void decoy() { // @decoy
  final t = src('conv-struct-field-boxed-decoy'); // @source(conv-struct-field-boxed-decoy)
  final req = Stmt(sql: 'constant');
  req.params['p'] = t;
  final Object v = req;
  sinkStmtAny(v); // @sink(none) @nosink(conv-struct-field-boxed-decoy)
  sinkCmd(t); // @sink(conv-struct-field-boxed-decoy)
}
