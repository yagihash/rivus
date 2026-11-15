part of 'lower.dart';

// Expressions. Each returns the local that holds the value.

int lowerExpr(FnLower f, Expression e) {
  if (f.cur == null) return f.nilConst(Pos.none);
  switch (e) {
    case ParenthesizedExpression():
      return lowerExpr(f, e.expression);
    case IntegerLiteral():
      return f.constant('int', '${e.value}', e.staticType, f.pos(e));
    case DoubleLiteral():
      return f.constant('float', '${e.value}', e.staticType, f.pos(e));
    case BooleanLiteral():
      return f.constant('bool', '${e.value}', e.staticType, f.pos(e));
    case NullLiteral():
      return f.nilConst(f.pos(e));
    case SimpleStringLiteral():
      return f.constant('string', e.value, e.staticType, f.pos(e));
    case AdjacentStrings():
      return _concat(f, [for (final s in e.strings) lowerExpr(f, s)], e);
    case StringInterpolation():
      return _interpolation(f, e);
    case SymbolLiteral():
      return f.constant('string', e.toSource(), e.staticType, f.pos(e));
    case ThisExpression():
      return f.readVar(thisKey, e.staticType, f.pos(e));
    case SuperExpression():
      return f.readVar(thisKey, e.staticType, f.pos(e));
    case SimpleIdentifier():
      return _identifier(f, e);
    case PrefixedIdentifier():
      return _prefixed(f, e);
    case PropertyAccess():
      return _propertyAccess(f, e);
    case MethodInvocation():
      return _methodInvocation(f, e);
    case FunctionExpressionInvocation():
      return _functionInvocation(f, e);
    case InstanceCreationExpression():
      return _creation(f, e);
    case AssignmentExpression():
      return _assignment(f, e);
    case BinaryExpression():
      return _binary(f, e);
    case PrefixExpression():
      return _prefix(f, e);
    case PostfixExpression():
      return _postfix(f, e);
    case IndexExpression():
      return _index(f, e);
    case ConditionalExpression():
      return _conditional(f, e);
    case AwaitExpression():
      return _await(f, e);
    case ThrowExpression():
      final v = lowerExpr(f, e.expression);
      f.thr(v, f.pos(e));
      return f.nilConst(Pos.none);
    case RethrowExpression():
      if (f.caught.isNotEmpty) f.thr(f.caught.last, f.pos(e));
      return f.nilConst(Pos.none);
    case AsExpression():
      final v = lowerExpr(f, e.expression);
      final d = f.tmp(e.staticType);
      f.emit('type_assert', dsts: [d], args: [v], at: f.pos(e), more: {'type': typeId(e.staticType)});
      return d;
    case IsExpression():
      final v = lowerExpr(f, e.expression);
      final tn = f.constant('string', e.type.toSource(), null, f.pos(e));
      final d = f.local('', null);
      f.out.locals[d] = Local('', 'dart:core#bool', 'bool');
      f.emit('binop', dsts: [d], args: [v, tn], at: f.pos(e), more: {'operator': e.notOperator == null ? 'is' : 'is!'});
      return d;
    case ListLiteral():
      return _collection(f, e, e.elements, isMap: false);
    case SetOrMapLiteral():
      return _collection(f, e, e.elements, isMap: e.isMap);
    case CascadeExpression():
      return _cascade(f, e);
    case FunctionExpression():
      return lowerClosure(f, e);
    case SwitchExpression():
      f.emit('unsupported', at: f.pos(e), more: {'reason': 'switch expression'});
      return f.nilConst(f.pos(e));
    default:
      f.emit('unsupported', at: f.pos(e), more: {'reason': 'expression ${e.runtimeType}'});
      return f.nilConst(f.pos(e));
  }
}

int _concat(FnLower f, List<int> parts, AstNode at) {
  if (parts.isEmpty) return f.constant('string', '', null, f.pos(at));
  var acc = parts.first;
  for (final p in parts.skip(1)) {
    final d = f.local('', null);
    f.out.locals[d] = Local('', 'dart:core#String', 'string');
    f.emit('binop', dsts: [d], args: [acc, p], at: f.pos(at), more: {'operator': '+'});
    acc = d;
  }
  return acc;
}

int _interpolation(FnLower f, StringInterpolation e) {
  final parts = <int>[];
  for (final el in e.elements) {
    switch (el) {
      case InterpolationString():
        if (el.value.isNotEmpty) parts.add(f.constant('string', el.value, null, f.pos(el)));
      case InterpolationExpression():
        parts.add(lowerExpr(f, el.expression));
    }
  }
  return _concat(f, parts, e);
}

// --- names ----------------------------------------------------------------

/// Whether the element is a variable of a function: a local or a parameter.
bool _isLocal(Element? el) => el is LocalVariableElement || el is FormalParameterElement && el is! FieldFormalParameterElement;

int _identifier(FnLower f, SimpleIdentifier e) {
  final el = e.element;
  final at = f.pos(e);
  if (el == null) return f.nilConst(at);
  if (_isLocal(el)) return f.readVar(el.baseElement, e.staticType, at);
  if (el is LocalFunctionElement) return f.readVar(el.baseElement, e.staticType, at);
  return _readMember(f, el, null, e, e.staticType, at);
}

int _prefixed(FnLower f, PrefixedIdentifier e) {
  final pre = e.prefix.element;
  final id = e.identifier.element;
  // library prefix or class name: a static member
  if (pre is PrefixElement || pre is InterfaceElement || pre is ExtensionElement) {
    return _readMember(f, id, null, e, e.staticType, f.pos(e, e.identifier.offset));
  }
  final base = lowerExpr(f, e.prefix);
  return _readMember(f, id, base, e, e.staticType, f.pos(e, e.identifier.offset));
}

int _propertyAccess(FnLower f, PropertyAccess e) {
  final t = e.target;
  final id = e.propertyName.element;
  int? base;
  if (t == null) {
    base = f.cascade;
  } else if (t is SimpleIdentifier && (t.element is PrefixElement || t.element is InterfaceElement)) {
    base = null;
  } else {
    base = lowerExpr(f, t);
  }
  return _readMember(f, id, base, e, e.staticType, f.pos(e, e.propertyName.offset));
}

/// Reads a member (a field, a getter, a tear-off) of [base], or of `this`
/// when there is none and the member is not static.
int _readMember(FnLower f, Element? el, int? base, AstNode n, DartType? t, Pos at) {
  if (el == null) return f.nilConst(at);
  var target = el.baseElement;
  if (target is PropertyAccessorElement && target.isOriginVariable) {
    target = target.variable.baseElement;
  }
  switch (target) {
    case FieldElement():
      if (target.isStatic) {
        return _globalLoad(f, target, t, at);
      }
      final b = base ?? f.readVar(thisKey, null, at);
      final d = f.tmp(t);
      final p = f.unit.fieldPos(n, at.offset);
      f.emit('field_load', dsts: [d], args: [b], at: p, more: {'field': target.name ?? ''});
      return d;
    case TopLevelVariableElement():
      return _globalLoad(f, target, t, at);
    case PropertyAccessorElement():
      // an explicit getter: a call
      return _call(f, target, base, const [], t, n, at, isGetter: true);
    case ExecutableElement():
      // a method or a function used as a value
      final d = f.tmp(t);
      final args = <int>[];
      if (!target.isStatic && target.enclosingElement is InterfaceElement) {
        args.add(base ?? f.readVar(thisKey, null, at));
      }
      f.emit('make_closure', dsts: [d], args: args, at: at, more: {'func': symbolOf(target)});
      return d;
    default:
      return f.nilConst(at);
  }
}

int _globalLoad(FnLower f, Element variable, DartType? t, Pos at) {
  final name = symbolOf(variable);
  f.prog.noteGlobal(name, typeId(t), Pos.none);
  final d = f.tmp(t);
  f.emit('global_load', dsts: [d], at: at, more: {'global': name});
  return d;
}

void _globalStore(FnLower f, Element variable, int value, Pos at) {
  final name = symbolOf(variable);
  f.prog.noteGlobal(name, '', Pos.none);
  f.emit('global_store', args: [value], at: at, more: {'global': name});
}

// --- calls ----------------------------------------------------------------

/// The arguments in the order of the parameters of the callee; what the call
/// leaves out is nil. Evaluated in the order of the source.
(List<int>, List<AstNode?>) _arguments(FnLower f, ExecutableElement? callee, ArgumentList list) {
  final params = callee == null ? const <FormalParameterElement>[] : orderedParams(callee);
  final slots = List<int?>.filled(params.length > list.arguments.length ? params.length : list.arguments.length, null);
  final nodes = List<AstNode?>.filled(slots.length, null);
  var next = 0;
  for (final a in list.arguments) {
    final x = a.argumentExpression;
    final v = lowerExpr(f, x);
    var idx = -1;
    final p = a.correspondingParameter?.baseElement;
    if (p != null) idx = params.indexOf(p);
    if (idx < 0) {
      while (next < slots.length && slots[next] != null) {
        next++;
      }
      idx = next;
    }
    if (idx < slots.length) {
      slots[idx] = v;
      nodes[idx] = x;
    }
  }
  final at = f.unit.at(list.offset);
  return ([for (final s in slots) s ?? f.nilConst(at)], nodes);
}

/// Emits a call. [base] is the receiver of an instance method.
int _call(FnLower f, ExecutableElement callee, int? base, List<int> args, DartType? resultType, AstNode n, Pos at,
    {bool isGetter = false, ArgumentList? argList, List<AstNode?> slots = const []}) {
  final el = callee.baseElement;
  final sym = symbolOf(el);
  final isMethod = el.enclosingElement is InterfaceElement && !el.isStatic && el is! ConstructorElement;
  final recv = isMethod ? (base ?? f.readVar(thisKey, null, at)) : null;
  final d = f.tmp(resultType);
  final Pos pos = argList == null ? at : f.unit.callPos(n, argList, slots);
  final all = [if (recv != null) recv, ...args];
  final callee_ = <String, Object?>{};
  // super calls and the like are static; the others go through the receiver
  final isSuper = n is MethodInvocation && n.target is SuperExpression;
  if (isMethod && !isSuper) {
    final cls = el.enclosingElement as InterfaceElement;
    callee_
      ..['kind'] = 'dynamic'
      ..['symbol'] = sym
      ..['candidates'] = f.prog.candidates(el, cls);
  } else {
    callee_
      ..['kind'] = 'static'
      ..['func'] = sym
      ..['symbol'] = sym;
    if (isMethod) callee_['recv'] = true;
  }
  final instr = f.emit('call', dsts: [d], args: all, at: pos, more: {'callee': callee_});
  f._lastCall = instr;
  f.mayThrow();
  return d;
}

int _methodInvocation(FnLower f, MethodInvocation e) {
  final el = e.methodName.element;
  final at = f.pos(e, e.argumentList.leftParenthesis.offset);
  // a function held in a variable
  if (el == null || _isLocal(el) || el is VariableElement) {
    final fn = e.target == null ? lowerExpr(f, e.methodName) : lowerExpr(f, e.function);
    return _callValue(f, fn, e.argumentList, e, e.staticType);
  }
  if (el is LocalFunctionElement) {
    final fn = f.readVar(el.baseElement, e.staticType, at);
    return _callValue(f, fn, e.argumentList, e, e.staticType);
  }
  if (el is! ExecutableElement) {
    return _callValue(f, lowerExpr(f, e.methodName), e.argumentList, e, e.staticType);
  }
  int? base;
  final t = e.target;
  if (t == null) {
    base = f.cascade;
  } else if (t is SimpleIdentifier && (t.element is PrefixElement || t.element is InterfaceElement || t.element is ExtensionElement)) {
    base = null;
  } else if (t is SuperExpression) {
    base = f.readVar(thisKey, null, at);
  } else {
    base = lowerExpr(f, t);
  }
  final (args, nodes) = _arguments(f, el, e.argumentList);
  return _call(f, el, base, args, e.staticType, e, at, argList: e.argumentList, slots: nodes);
}

int _functionInvocation(FnLower f, FunctionExpressionInvocation e) {
  final fn = lowerExpr(f, e.function);
  return _callValue(f, fn, e.argumentList, e, e.staticType);
}

int _callValue(FnLower f, int fn, ArgumentList list, AstNode n, DartType? t) {
  final args = <int>[];
  final nodes = <AstNode?>[];
  for (final a in list.arguments) {
    final x = a.argumentExpression;
    args.add(lowerExpr(f, x));
    nodes.add(x);
  }
  final d = f.tmp(t);
  final pos = f.unit.callPos(n, list, nodes);
  final instr = f.emit('call', dsts: [d], args: [fn, ...args], at: pos, more: {
    'callee': {'kind': 'value'}
  });
  f._lastCall = instr;
  f.mayThrow();
  return d;
}

int _creation(FnLower f, InstanceCreationExpression e) {
  final ctor = e.constructorName.element;
  final at = f.pos(e, e.argumentList.leftParenthesis.offset);
  if (ctor == null) {
    for (final a in e.argumentList.arguments) {
      lowerExpr(f, a.argumentExpression);
    }
    return f.nilConst(at);
  }
  final (args, nodes) = _arguments(f, ctor, e.argumentList);
  final d = f.tmp(e.staticType);
  final pos = f.unit.callPos(e, e.argumentList, nodes);
  final sym = symbolOf(ctor);
  final instr = f.emit('call', dsts: [d], args: args, at: pos, more: {
    'callee': {'kind': 'static', 'func': sym, 'symbol': sym}
  });
  f._lastCall = instr;
  f.mayThrow();
  return d;
}

// --- operators ------------------------------------------------------------

int _binary(FnLower f, BinaryExpression e) {
  final op = e.operator.type;
  if (op == TokenType.AMPERSAND_AMPERSAND || op == TokenType.BAR_BAR) {
    final t = f.newBlock();
    final fl = f.newBlock();
    final join = f.newBlock();
    final r = f.local('', null);
    f.out.locals[r] = Local('', 'dart:core#bool', 'bool');
    f.cond(e, t, fl);
    f.cur = t;
    f.emit('const', dsts: [r], more: {
      'const': {'kind': 'bool', 'value': 'true'}
    });
    f.jump(join);
    f.cur = fl;
    f.emit('const', dsts: [r], more: {
      'const': {'kind': 'bool', 'value': 'false'}
    });
    f.jump(join);
    f.cur = join;
    return r;
  }
  if (op == TokenType.QUESTION_QUESTION) {
    final l = lowerExpr(f, e.leftOperand);
    final r = f.tmp(e.staticType);
    f.emit('copy', dsts: [r], args: [l], at: f.pos(e));
    final nilc = f.nilConst(f.pos(e));
    final isNull = f.local('', null);
    f.out.locals[isNull] = Local('', 'dart:core#bool', 'bool');
    f.emit('binop', dsts: [isNull], args: [l, nilc], at: f.pos(e), more: {'operator': '=='});
    final rhs = f.newBlock();
    final join = f.newBlock();
    f.branch(isNull, rhs, join, f.pos(e));
    f.cur = rhs;
    final v = lowerExpr(f, e.rightOperand);
    f.emit('copy', dsts: [r], args: [v], at: f.pos(e));
    f.jump(join);
    f.cur = join;
    return r;
  }
  final l = lowerExpr(f, e.leftOperand);
  final r = lowerExpr(f, e.rightOperand);
  final d = f.tmp(e.staticType);
  f.emit('binop', dsts: [d], args: [l, r], at: f.pos(e, e.operator.offset), more: {'operator': e.operator.lexeme});
  return d;
}

int _prefix(FnLower f, PrefixExpression e) {
  final op = e.operator.type;
  if (op == TokenType.PLUS_PLUS || op == TokenType.MINUS_MINUS) {
    final v = lowerExpr(f, e.operand);
    final one = f.constant('int', '1', null, f.pos(e));
    final d = f.tmp(e.staticType);
    f.emit('binop', dsts: [d], args: [v, one], at: f.pos(e), more: {'operator': op == TokenType.PLUS_PLUS ? '+' : '-'});
    lowerAssign(f, e.operand, d);
    return d;
  }
  final v = lowerExpr(f, e.operand);
  final d = f.tmp(e.staticType);
  f.emit('unop', dsts: [d], args: [v], at: f.pos(e), more: {'operator': e.operator.lexeme});
  return d;
}

int _postfix(FnLower f, PostfixExpression e) {
  final op = e.operator.type;
  if (op == TokenType.PLUS_PLUS || op == TokenType.MINUS_MINUS) {
    final v = lowerExpr(f, e.operand);
    final one = f.constant('int', '1', null, f.pos(e));
    final d = f.tmp(e.staticType);
    f.emit('binop', dsts: [d], args: [v, one], at: f.pos(e), more: {'operator': op == TokenType.PLUS_PLUS ? '+' : '-'});
    lowerAssign(f, e.operand, d);
    return v;
  }
  // x! : the value itself
  return lowerExpr(f, e.operand);
}

int _conditional(FnLower f, ConditionalExpression e) {
  final t = f.newBlock();
  final fl = f.newBlock();
  final join = f.newBlock();
  final r = f.tmp(e.staticType);
  f.cond(e.condition, t, fl);
  f.cur = t;
  var v = lowerExpr(f, e.thenExpression);
  f.emit('copy', dsts: [r], args: [v], at: f.pos(e));
  f.jump(join);
  f.cur = fl;
  v = lowerExpr(f, e.elseExpression);
  f.emit('copy', dsts: [r], args: [v], at: f.pos(e));
  f.jump(join);
  f.cur = join;
  return r;
}

int _await(FnLower f, AwaitExpression e) {
  final v = lowerExpr(f, e.expression);
  final last = f._lastCall;
  if (last != null && (last['dsts'] as List?)?.first == v) last['await'] = true;
  final next = f.newBlock();
  f.jump(next);
  f.cur = next;
  return v;
}

// --- assignment -----------------------------------------------------------

int _assignment(FnLower f, AssignmentExpression e) {
  final op = e.operator.type;
  int value;
  if (op == TokenType.EQ) {
    value = lowerExpr(f, e.rightHandSide);
  } else if (op == TokenType.QUESTION_QUESTION_EQ) {
    value = lowerExpr(f, e.rightHandSide);
  } else {
    final cur = lowerExpr(f, e.leftHandSide);
    final r = lowerExpr(f, e.rightHandSide);
    final d = f.tmp(e.staticType);
    final lex = e.operator.lexeme;
    f.emit('binop', dsts: [d], args: [cur, r], at: f.pos(e), more: {'operator': lex.substring(0, lex.length - 1)});
    value = d;
  }
  lowerAssign(f, e.leftHandSide, value);
  return value;
}

void lowerAssign(FnLower f, Expression target, int value) {
  var x = target;
  while (x is ParenthesizedExpression) {
    x = x.expression;
  }
  final at = f.pos(x);
  switch (x) {
    case SimpleIdentifier():
      final el = x.element;
      final w = x.parent is AssignmentExpression ? (x.parent as AssignmentExpression).writeElement : el;
      final we = w ?? el;
      if (we == null) return;
      if (_isLocal(we)) {
        f.writeVar(we.baseElement, x.staticType, value, at);
      } else {
        _writeMember(f, we, null, x, value, at);
      }
    case PrefixedIdentifier():
      final pre = x.prefix.element;
      final we = (x.parent is AssignmentExpression ? (x.parent as AssignmentExpression).writeElement : null) ?? x.identifier.element;
      if (pre is PrefixElement || pre is InterfaceElement) {
        _writeMember(f, we, null, x, value, f.pos(x, x.identifier.offset));
      } else {
        final base = lowerExpr(f, x.prefix);
        _writeMember(f, we, base, x, value, f.pos(x, x.identifier.offset));
      }
    case PropertyAccess():
      final we = (x.parent is AssignmentExpression ? (x.parent as AssignmentExpression).writeElement : null) ?? x.propertyName.element;
      final t = x.target;
      final base = t == null ? f.cascade : lowerExpr(f, t);
      _writeMember(f, we, base, x, value, f.pos(x, x.propertyName.offset));
    case IndexExpression():
      final t = x.target;
      final base = t == null ? f.cascade! : lowerExpr(f, t);
      final idx = lowerExpr(f, x.index);
      final isMap = _isMap(x.target?.staticType);
      f.emit('index_store', args: [base, idx, value], at: at, more: {if (isMap) 'keyed': true});
    default:
      f.emit('unsupported', at: at, more: {'reason': 'assignment to ${x.runtimeType}'});
  }
}

bool _isMap(DartType? t) => t is InterfaceType && t.element.name == 'Map';

void _writeMember(FnLower f, Element? el, int? base, AstNode n, int value, Pos at) {
  if (el == null) return;
  var target = el.baseElement;
  if (target is PropertyAccessorElement && target.isOriginVariable) target = target.variable.baseElement;
  switch (target) {
    case FieldElement():
      if (target.isStatic) {
        _globalStore(f, target, value, at);
        return;
      }
      final b = base ?? f.readVar(thisKey, null, at);
      f.emit('field_store', args: [b, value], at: f.unit.fieldPos(n, at.offset), more: {'field': target.name ?? ''});
    case TopLevelVariableElement():
      _globalStore(f, target, value, at);
    case PropertyAccessorElement():
      // an explicit setter: a call
      _call(f, target, base, [value], null, n, at);
    default:
  }
}

// --- indexing and collections ---------------------------------------------

int _index(FnLower f, IndexExpression e) {
  final t = e.target;
  final base = t == null ? f.cascade! : lowerExpr(f, t);
  final idx = lowerExpr(f, e.index);
  final d = f.tmp(e.staticType);
  final isMap = _isMap(t?.staticType);
  f.emit('index_load', dsts: [d], args: [base, idx], at: f.pos(e), more: {if (isMap) 'keyed': true});
  return d;
}

int _collection(FnLower f, TypedLiteral e, NodeList<CollectionElement> elements, {required bool isMap}) {
  final d = f.tmp(e.staticType);
  f.emit('alloc', dsts: [d], at: f.pos(e), more: {'type': typeId(e.staticType), 'class': 'container'});
  var n = 0;
  void add(CollectionElement el) {
    switch (el) {
      case MapLiteralEntry():
        final k = lowerExpr(f, el.key);
        final v = lowerExpr(f, el.value);
        f.emit('index_store', args: [d, k, v], at: f.pos(el), more: {'keyed': true});
      case Expression():
        final v = lowerExpr(f, el);
        final i = f.constant('int', '${n++}', null, f.pos(el));
        f.emit('index_store', args: [d, i, v], at: f.pos(el), more: isMap ? {'keyed': true} : {});
      case SpreadElement():
        final v = lowerExpr(f, el.expression);
        final i = f.constant('int', '${n++}', null, f.pos(el));
        f.emit('index_store', args: [d, i, v], at: f.pos(el));
      default:
        f.emit('unsupported', at: f.pos(el), more: {'reason': 'collection element ${el.runtimeType}'});
    }
  }

  for (final el in elements) {
    if (el is IfElement) {
      // Both branches may contribute: lower them in sequence, after the test.
      lowerExpr(f, el.expression);
      add(el.thenElement);
      final other = el.elseElement;
      if (other != null) add(other);
    } else if (el is ForElement) {
      f.emit('unsupported', at: f.pos(el), more: {'reason': 'collection for'});
    } else {
      add(el);
    }
  }
  return d;
}

int _cascade(FnLower f, CascadeExpression e) {
  final base = lowerExpr(f, e.target);
  final saved = f.cascade;
  f.cascade = base;
  for (final s in e.cascadeSections) {
    lowerExpr(f, s);
  }
  f.cascade = saved;
  return base;
}

// --- closures -------------------------------------------------------------

int lowerClosure(FnLower f, FunctionExpression fe, {String? name}) {
  final id = '${f.out.id}\$${f.nextClosure()}';
  final out = FuncOut(id)..symbol = '';
  out.pos = f.pos(fe);
  final child = FnLower(f.prog, f.unit, out, parent: f, cells: f.cells, assigned: f.assigned);
  final params = <(Object, String, DartType?)>[];
  final fel = fe.declaredFragment?.element;
  for (final p in fel == null ? const <FormalParameterElement>[] : orderedParams(fel)) {
    params.add((p, p.name ?? '', p.type));
  }
  child.lowerBody(params: params, body: fe.body, at: out.pos);
  f.unit.funcTexts(id, fe);
  f.prog.out.func(out);
  final d = f.tmp(fe.staticType);
  final args = <int>[];
  for (final key in child.captures) {
    final l = f._lookup(key, null, wantCell: false);
    if (l != null) args.add(l);
  }
  f.emit('make_closure', dsts: [d], args: args, at: out.pos, more: {'func': id});
  return d;
}
