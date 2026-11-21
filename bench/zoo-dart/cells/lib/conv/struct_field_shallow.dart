// @cell(F/conv/struct-field-shallow)
// @shape: a statement (the sql is the dangerous field, the bind params are not) is given whole to the call that runs it, and the rule of the sink names the field (docs/DESIGN-field-sinks.md)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/struct-field-shallow/vuln', vuln);
  zooCase('F/conv/struct-field-shallow/fixed', fixed);
  zooCase('F/conv/struct-field-shallow/decoy', decoy);
}

void vuln() { // @vuln
  final req = Stmt();
  req.sql = src('conv-struct-field-shallow-vuln'); // @source(conv-struct-field-shallow-vuln)
  sinkStmt(req); // @sink(conv-struct-field-shallow-vuln)
}

void fixed() { // @fixed
  final req = Stmt();
  req.sql = cleanSql(src('conv-struct-field-shallow-fixed'));
  sinkStmt(req); // @sink(none)
}

void decoy() { // @decoy
  final t = src('conv-struct-field-shallow-decoy'); // @source(conv-struct-field-shallow-decoy)
  final req = Stmt(sql: 'constant');
  req.params['p'] = t;
  sinkStmt(req); // @sink(none) @nosink(conv-struct-field-shallow-decoy)
  sinkCmd(t); // @sink(conv-struct-field-shallow-decoy)
}
