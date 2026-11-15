part of 'lower.dart';

// Switch expressions and records.

int _switchExpression(FnLower f, SwitchExpression e) {
  final subject = lowerExpr(f, e.expression);
  final r = f.tmp(e.staticType);
  final join = f.newBlock();
  for (final c in e.cases) {
    final miss = f.newBlock();
    f.bindGuarded(c.guardedPattern, subject, miss);
    final v = lowerExpr(f, c.expression);
    f.emit('copy', dsts: [r], args: [v], at: f.pos(c));
    f.jump(join);
    f.cur = miss;
  }
  // no case matched
  f.thr(f.constant('string', 'no case matched', null, f.pos(e)), f.pos(e));
  f.cur = join;
  return r;
}

/// A record is a value with fields named $1, $2, ... and by name.
int _record(FnLower f, RecordLiteral e) {
  final d = f.tmp(e.staticType);
  f.emit('alloc', dsts: [d], at: f.pos(e), more: {'type': 'record', 'class': 'struct'});
  var i = 0;
  for (final field in e.fields) {
    final named = field.beginToken.next?.lexeme == ':';
    final name = named ? field.beginToken.lexeme : positionalName(++i);
    final v = lowerExpr(f, field.fieldExpression);
    f.emit('field_store', args: [d, v], at: f.pos(field), more: {'field': name});
  }
  return d;
}

/// The name of the i-th positional field of a record.
String positionalName(int i) => '\$$i';
