// @cell(F/conv/struct-field-opaque)
// @shape: a sibling of the dangerous field is tainted, and the structure goes through a function that the analysis has no body and no model of, which moves it into the dangerous field (docs/DESIGN-field-sinks.md)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/struct-field-opaque/vuln', vuln);
  zooCase('F/conv/struct-field-opaque/fixed', fixed);
  zooCase('F/conv/struct-field-opaque/decoy', decoy);
}

void vuln() { // @vuln
  final req = Stmt();
  req.params['body'] = src('conv-struct-field-opaque-vuln'); // @source(conv-struct-field-opaque-vuln)
  opaque(req);
  sinkStmt(req); // @sink(conv-struct-field-opaque-vuln)
}

void fixed() { // @fixed
  final req = Stmt();
  req.params['body'] = cleanSql(src('conv-struct-field-opaque-fixed'));
  opaque(req);
  sinkStmt(req); // @sink(none)
}

void decoy() { // @decoy
  final t = src('conv-struct-field-opaque-decoy'); // @source(conv-struct-field-opaque-decoy)
  final req = Stmt(sql: 'constant');
  req.params['body'] = t;
  final other = Stmt();
  opaque(other);
  sinkStmt(req); // @sink(none) @nosink(conv-struct-field-opaque-decoy)
  sinkCmd(t); // @sink(conv-struct-field-opaque-decoy)
}
