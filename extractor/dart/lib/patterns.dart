part of 'lower.dart';

// Patterns (Dart 3): a pattern is tests on a value, reads of its parts, and
// the variables it binds. A test that fails goes to [fail]; the code after
// the pattern is the code where it matched. Exhaustiveness is not reasoned
// about: the edge for "nothing matched" stays.

extension PatternLowering on FnLower {
  void _bindVar(Object key, String name, DartType? t, int value, Pos at) {
    if (vars.containsKey(key) || cellVars.containsKey(key)) {
      writeVar(key, t, value, at);
    } else {
      declare(key, name, t, value, at);
    }
  }

  /// Branches on a test: on to the code that follows when it holds, to
  /// [fail] when it does not.
  void _test(int cond, Blk fail, Pos at) {
    if (cur == null) return;
    final ok = newBlock();
    branch(cond, ok, fail, at);
    cur = ok;
  }

  int _boolLocal() {
    final d = local('', null);
    out.locals[d] = Local('', 'dart:core#bool', 'bool');
    return d;
  }

  int _isType(int v, String type, Pos at) {
    final tn = constant('string', type, null, at);
    final d = _boolLocal();
    emit('binop', dsts: [d], args: [v, tn], at: at, more: {'operator': 'is'});
    return d;
  }

  void bindPattern(DartPattern p, int v, Blk fail) {
    if (cur == null) return;
    final at = pos(p);
    switch (p) {
      case ParenthesizedPattern():
        bindPattern(p.pattern, v, fail);
      case DeclaredVariablePattern():
        final el = p.declaredFragment?.element;
        if (el == null) {
          emit('unsupported', at: at, more: {'reason': 'a variable of a pattern that does not resolve: ${p.name.lexeme}'});
          return;
        }
        _bindVar(el.baseElement, p.name.lexeme, el.type, v, at);
      case AssignedVariablePattern():
        final el = p.element;
        if (el != null && _isLocal(el)) {
          writeVar(el.baseElement, null, v, at);
        } else {
          emit('unsupported', at: at, more: {'reason': 'a pattern that assigns to ${el == null ? 'a name that does not resolve' : 'a variable that is not local'}: ${p.toSource()}'});
        }
      case WildcardPattern():
        break;
      case ConstantPattern():
        final c = expr(p.expression);
        final eq = _boolLocal();
        emit('binop', dsts: [eq], args: [v, c], at: at, more: {'operator': '=='});
        _test(eq, fail, at);
      case RelationalPattern():
        final c = expr(p.operand);
        final r = _boolLocal();
        emit('binop', dsts: [r], args: [v, c], at: at, more: {'operator': p.operator.lexeme});
        _test(r, fail, at);
      case CastPattern():
        final d = tmp(p.type.type);
        emit('type_assert', dsts: [d], args: [v], at: at, more: {'type': typeId(p.type.type)});
        bindPattern(p.pattern, d, fail);
      case NullCheckPattern():
        bindPattern(p.pattern, v, fail);
      case NullAssertPattern():
        bindPattern(p.pattern, v, fail);
      case LogicalAndPattern():
        bindPattern(p.leftOperand, v, fail);
        bindPattern(p.rightOperand, v, fail);
      case LogicalOrPattern():
        // Either side may match; both bind the same variables.
        final right = newBlock();
        final join = newBlock();
        bindPattern(p.leftOperand, v, right);
        jump(join);
        cur = right;
        bindPattern(p.rightOperand, v, fail);
        jump(join);
        cur = join;
      case ObjectPattern():
        final t = p.type.type;
        if (t != null) _test(_isType(v, typeId(t), at), fail, at);
        for (final f in p.fields) {
          final part = _patternField(f, v);
          bindPattern(f.pattern, part, fail);
        }
      case RecordPattern():
        var i = 0;
        for (final f in p.fields) {
          final name = f.name?.name?.lexeme ?? f.effectiveName ?? positionalName(++i);
          final d = tmp(null);
          emit('field_load', dsts: [d], args: [v], at: pos(f), more: {'field': name});
          bindPattern(f.pattern, d, fail);
        }
      case ListPattern():
        var i = 0;
        for (final e in p.elements) {
          if (e is RestPatternElement) {
            // the rest is a list of the same elements
            final sub = e.pattern;
            if (sub != null) bindPattern(sub, v, fail);
            continue;
          }
          final idx = constant('int', '${i++}', null, at);
          final d = tmp(null);
          emit('index_load', dsts: [d], args: [v, idx], at: pos(e));
          bindPattern(e as DartPattern, d, fail);
        }
      case MapPattern():
        for (final e in p.elements) {
          if (e is MapPatternEntry) {
            final k = expr(e.key);
            final d = tmp(null);
            emit('index_load', dsts: [d], args: [v, k], at: pos(e), more: {'keyed': true});
            bindPattern(e.value, d, fail);
          } else if (e is RestPatternElement) {
            final sub = e.pattern;
            if (sub != null) bindPattern(sub, v, fail);
          }
        }
    }
  }

  /// The value of a field an object pattern reads: a getter or a field.
  int _patternField(PatternField f, int base) {
    final at = pos(f);
    final el = f.element;
    if (el != null) {
      return _readMember(this, el, base, f, null, at);
    }
    final d = tmp(null);
    emit('field_load', dsts: [d], args: [base], at: at, more: {'field': f.effectiveName ?? ''});
    return d;
  }

  /// A guarded pattern: matched, and the guard holds; otherwise [fail].
  void bindGuarded(GuardedPattern g, int v, Blk fail) {
    bindPattern(g.pattern, v, fail);
    final w = g.whenClause;
    if (w != null && cur != null) {
      final ok = newBlock();
      cond(w.expression, ok, fail);
      cur = ok;
    }
  }
}
