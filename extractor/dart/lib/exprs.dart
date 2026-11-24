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
      // a cast is not a conversion: the value is the same one, seen as another type (the
      // engine's type_assert reads the value out of an interface box, which Dart has not)
      f.emit('copy', dsts: [d], args: [v], at: f.pos(e));
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
      return _switchExpression(f, e);
    case RecordLiteral():
      return _record(f, e);
    case PatternAssignment():
      final v = lowerExpr(f, e.expression);
      final miss = f.newBlock();
      f.bindPattern(e.pattern, v, miss);
      final matched = f.newBlock();
      f.jump(matched);
      f.cur = miss;
      f.thr(f.constant('string', 'pattern did not match', null, f.pos(e)), f.pos(e));
      f.cur = matched;
      return v;
    case ExtensionOverride():
      // `Ext(x).m()`: the receiver is the one argument
      final args = e.argumentList.arguments;
      return args.isEmpty ? f.nilConst(f.pos(e)) : lowerExpr(f, args.first.argumentExpression);
    case FunctionReference():
      // `f<T>`: a tear-off with its type arguments, the same function
      return lowerExpr(f, e.function);
    case ConstructorReference():
      // `C.new`, `C<T>.named`: a tear-off of the constructor
      final ctor = e.constructorName.element;
      if (ctor == null) {
        f.emit('unsupported', at: f.pos(e), more: {'reason': 'a constructor tear-off that does not resolve: ${e.constructorName.toSource()}'});
        return f.nilConst(f.pos(e));
      }
      final d = f.tmp(e.staticType);
      f.emit('make_closure', dsts: [d], args: const [], at: f.pos(e), more: {'func': funcId(ctor)});
      return d;
    case TypeLiteral():
      return f.constant('string', e.toSource(), e.staticType, f.pos(e));
    case DotShorthandPropertyAccess():
      // `.red`: a static member of the type the context asks for
      return _readMember(f, e.propertyName.element, null, e, e.staticType, f.pos(e, e.propertyName.offset));
    case DotShorthandInvocation():
      return _dotShorthandInvocation(f, e);
    case DotShorthandConstructorInvocation():
      return _dotShorthandConstructor(f, e);
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
// a `this.x` parameter is a parameter too: the initializer list and the body read it as one
bool _isLocal(Element? el) => el is LocalVariableElement || el is FormalParameterElement;

int _identifier(FnLower f, SimpleIdentifier e) {
  final el = e.element ?? _readElementOf(e);
  final at = f.pos(e);
  if (el == null) {
    // `$1`, `$2` in an extension of a record type: the field of this
    if (RegExp(r'^\$\d+$').hasMatch(e.name)) return _recordField(f, f.readVar(thisKey, null, at), e.name, e.staticType, at);
    f.emit('unsupported', at: at, more: {'reason': 'a name that does not resolve: ${e.name}'});
    return f.nilConst(at);
  }
  if (_isLocal(el)) return f.readVar(el.baseElement, e.staticType, at);
  if (el is LocalFunctionElement) return f.readVar(el.baseElement, e.staticType, at);
  return _readMember(f, el, null, e, e.staticType, at);
}

int _prefixed(FnLower f, PrefixedIdentifier e) {
  final pre = e.prefix.element;
  final id = e.identifier.element ?? _readElementOf(e);
  // library prefix or class name: a static member
  if (pre is PrefixElement || pre is InterfaceElement || pre is ExtensionElement) {
    return _readMember(f, id, null, e, e.staticType, f.pos(e, e.identifier.offset));
  }
  final base = lowerExpr(f, e.prefix);
  if (id == null && e.prefix.staticType is RecordType) {
    return _recordField(f, base, e.identifier.name, e.staticType, f.pos(e, e.identifier.offset));
  }
  return _readMember(f, id, base, e, e.staticType, f.pos(e, e.identifier.offset), dynamicName: e.identifier.name);
}

int _propertyAccess(FnLower f, PropertyAccess e) {
  final t = e.target;
  final id = e.propertyName.element ?? _readElementOf(e);
  int? base;
  if (t == null) {
    base = f.cascade;
  } else if (t is SimpleIdentifier && (t.element is PrefixElement || t.element is InterfaceElement)) {
    base = null;
  } else {
    base = lowerExpr(f, t);
  }
  if (id == null && base != null && t?.staticType is RecordType) {
    return _recordField(f, base, e.propertyName.name, e.staticType, f.pos(e, e.propertyName.offset));
  }
  return _readMember(f, id, base, e, e.staticType, f.pos(e, e.propertyName.offset), dynamicName: e.propertyName.name);
}

int _recordField(FnLower f, int base, String name, DartType? t, Pos at) {
  final d = f.tmp(t);
  f.emit('field_load', dsts: [d], args: [base], at: at, more: {'field': name});
  return d;
}

/// Reads a member (a field, a getter, a tear-off) of [base], or of `this`
/// when there is none and the member is not static.
int _readMember(FnLower f, Element? el, int? base, AstNode n, DartType? t, Pos at, {String? dynamicName}) {
  if (el == null) {
    // a member of a dynamic value: a field of it by name (what it holds is what the
    // member is, more than the truth and never less)
    if (base != null && dynamicName != null) return _recordField(f, base, dynamicName, t, at);
    f.emit('unsupported', at: at, more: {'reason': 'a name that does not resolve: ${dynamicName ?? n.toSource()}'});
    return f.nilConst(at);
  }
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
      if (!target.isStatic && target is! ConstructorElement && (target.enclosingElement is InterfaceElement || target.enclosingElement is ExtensionElement)) {
        // an instance method as a value: a bound method, a closure that captures the receiver and
        // calls the method on it (through the receiver's type, as a call does)
        final recv = base ?? f.readVar(thisKey, null, at);
        final id = _boundMethod(f, target, n);
        f.emit('make_closure', dsts: [d], args: [recv], at: at, more: {'func': id});
        return d;
      }
      f.emit('make_closure', dsts: [d], args: const [], at: at, more: {'func': funcId(target)});
      return d;
    default:
      return f.nilConst(at);
  }
}

/// The function that stands for `recv.method` used as a value: it takes the arguments of the method, holds
/// the receiver as what it captured, and calls the method on it. One for each method (the receiver is
/// what a closure made of it captures). Returns its id.
String _boundMethod(FnLower f, ExecutableElement method, AstNode n) {
  final m = method.baseElement;
  final id = '${funcId(m)}\$bound${f.out.dep ? '\$dep' : ''}';
  if (!f.prog.boundDone.add(id)) return id;
  final out = FuncOut(id)
    ..symbol = ''
    ..dep = f.out.dep;
  final at = f.pos(n);
  out.pos = at;
  final child = FnLower(f.prog, f.unit, out, cells: const {}, assigned: const {});
  child.cur = child.newBlock();
  final params = orderedParams(m);
  final args = <int>[];
  for (var i = 0; i < params.length; i++) {
    final p = params[i];
    final l = child.local(p.name ?? '', p.type);
    out.params.add(Param(p.name ?? '', typeId(p.type), typeClass(p.type), displayType(p.type)));
    child.emit('param', dsts: [l], at: at, more: {'index': i});
    args.add(l);
  }
  final recv = child.local('', null);
  child.captures.add(thisKey);
  child.captureLocal[thisKey] = recv;
  child.vars[thisKey] = recv;
  final r = _call(child, m, recv, args, null, n, at);
  child.ret([r], Pos.none);
  child.finishCaptures();
  f.prog.out.func(out);
  return id;
}

int _globalLoad(FnLower f, Element variable, DartType? t, Pos at) {
  final name = symbolOf(variable);
  f.prog.noteGlobal(name, typeId(t), Pos.none);
  final d = f.tmp(t);
  f.emit('global_load', dsts: [d], at: at, more: {'global': name});
  return d;
}

void globalStore(FnLower f, Element variable, int value, Pos at) => _globalStore(f, variable, value, at);

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

/// The names of the named parameters that the arguments are for, parallel to them ("" for a positional one):
/// a rule that says `named: kind` finds the argument by it. Null when the callee has no named parameter.
List<String>? _argNames(ExecutableElement? callee, int n, {bool recv = false}) {
  if (callee == null) return null;
  final params = orderedParams(callee);
  if (!params.any((p) => p.isNamed)) return null;
  return [
    if (recv) '',
    for (var i = 0; i < n; i++) i < params.length && params[i].isNamed ? (params[i].name ?? '') : '',
  ];
}

/// Emits a call. [base] is the receiver of an instance method.
int _call(FnLower f, ExecutableElement callee, int? base, List<int> args, DartType? resultType, AstNode n, Pos at,
    {bool isGetter = false, ArgumentList? argList, List<AstNode?> slots = const []}) {
  final el = callee.baseElement;
  final sym = symbolOf(el);
  final isMethod = el.enclosingElement is InterfaceElement && !el.isStatic && el is! ConstructorElement;
  // an extension member is a function that takes the receiver first
  final isExt = el.enclosingElement is ExtensionElement && !el.isStatic;
  final recv = isMethod || isExt ? (base ?? f.readVar(thisKey, null, at)) : null;
  final d = f.tmp(resultType);
  final Pos pos = argList == null ? at : f.unit.callPos(n, argList, slots);
  final all = [if (recv != null) recv, ...args];
  final callee_ = <String, Object?>{};
  // super calls and the like are static; the others go through the receiver
  // `super.m()`, `super.x` and `super.x = v` run the member of the direct superclass: not the ones that a subclass (this
  // one, for example) overrides it with, which is what a dynamic call would also reach
  final isSuper = (n is MethodInvocation && n.target is SuperExpression) || (n is PropertyAccess && n.target is SuperExpression);
  if (isMethod && !isSuper) {
    final cls = el.enclosingElement as InterfaceElement;
    callee_
      ..['kind'] = 'dynamic'
      ..['symbol'] = sym
      ..['candidates'] = f.prog.candidates(el, cls);
  } else {
    callee_
      ..['kind'] = 'static'
      ..['func'] = funcId(el)
      ..['symbol'] = sym;
    if (isMethod || isExt) callee_['recv'] = true;
  }
  final names = _argNames(el, args.length, recv: recv != null);
  final instr = f.emit('call', dsts: [d], args: all, at: pos, more: {'callee': callee_, if (names != null) 'arg_names': names});
  f._lastCall = instr;
  f.mayThrow();
  return d;
}

int _methodInvocation(FnLower f, MethodInvocation e) {
  final el = e.methodName.element;
  final at = f.pos(e, e.argumentList.leftParenthesis.offset);
  // a method of a value whose type says nothing about it (dynamic), or `f.call(x)` / `f?.call(x)` of a
  // function value: the receiver is what is called, or holds the member that is
  final target = e.target;
  if (el == null && target != null) {
    final base = lowerExpr(f, target);
    if (e.methodName.name == 'call') return _callValue(f, base, e.argumentList, e, e.staticType);
    // what the method is, is not known: what it returns is taken to come from the receiver and the arguments
    final args = <int>[base];
    for (final a in e.argumentList.arguments) {
      args.add(lowerExpr(f, a.argumentExpression));
    }
    final d = f.tmp(e.staticType);
    final instr = f.emit('call', dsts: [d], args: args, at: at, more: {
      'callee': {'kind': 'static', 'symbol': 'dart:core#dynamic.invoke'}
    });
    f._lastCall = instr;
    f.mayThrow();
    return d;
  }
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
    // a section of a cascade works on its target; a call that has no target and is not a section is a
    // call of this (`..write(_header())`: _header is a method of the class around, not of the buffer)
    base = e.isCascaded ? f.cascade : null;
  } else if (t is SimpleIdentifier && (t.element is PrefixElement || t.element is InterfaceElement || t.element is ExtensionElement)) {
    base = null;
  } else if (t is SuperExpression) {
    base = f.readVar(thisKey, null, at);
  } else {
    base = lowerExpr(f, t);
  }
  final (args, nodes) = _arguments(f, el, e.argumentList);
  final result = _call(f, el, base, args, e.staticType, e, at, argList: e.argumentList, slots: nodes);
  // an API that calls the function it is given with what the receiver holds
  final mode = _callbackMode(el);
  if (mode != null && base != null && args.isNotEmpty) _callWhatTheReceiverHolds(f, mode, base, args[0], at);
  return result;
}

/// The functions of the platform that call the function they are given (the first argument)
/// with a value the receiver holds: nothing in the code says so, and the models only
/// move taint from arguments to results, so the extractor makes the call.
String? _callbackMode(ExecutableElement m) {
  final cls = m.enclosingElement;
  if (cls is! InterfaceElement) return null;
  final owner = '${libraryUri(cls)}#${cls.name}';
  switch ('$owner.${m.name}') {
    case 'dart:async#Future.then':
      return 'value';
    case 'dart:async#Stream.listen':
    case 'dart:async#Stream.forEach':
    case 'dart:core#Iterable.forEach':
    case 'dart:core#Iterable.map':
    case 'dart:core#Iterable.where':
    case 'dart:core#Iterable.any':
    case 'dart:core#Iterable.every':
    case 'dart:core#Iterable.expand':
    case 'dart:core#Iterable.firstWhere':
    case 'dart:core#Iterable.singleWhere':
    case 'dart:core#Iterable.lastWhere':
    case 'dart:core#Iterable.takeWhile':
    case 'dart:core#Iterable.skipWhile':
      return 'element';
  }
  return null;
}

/// `receiver.api(callback)`: the call that the platform makes, of the callback with what
/// the receiver holds (the value of a future, an element of a stream or a collection).
void _callWhatTheReceiverHolds(FnLower f, String mode, int receiver, int callback, Pos at) {
  var arg = receiver;
  if (mode == 'element') {
    final it = f.tmp(null);
    f.emit('call', dsts: [it], args: [receiver], at: at, more: {
      'callee': {'kind': 'static', 'symbol': 'iter.start'}
    });
    final ok = f.local('', null);
    final k = f.local('', null);
    final v = f.local('', null);
    f.out.locals[ok] = Local('', 'dart:core#bool', 'bool');
    f.emit('call', dsts: [ok, k, v], args: [it], at: at, more: {
      'callee': {'kind': 'static', 'symbol': 'iter.next'}
    });
    arg = v;
  }
  final d = f.tmp(null);
  f.emit('call', dsts: [d], args: [callback, arg], at: at, more: {
    'callee': {'kind': 'value'}
  });
}

/// `.make(3)`: a static method (or a constructor) of the type the context asks for.
int _dotShorthandInvocation(FnLower f, DotShorthandInvocation e) {
  final el = e.memberName.element;
  final at = f.pos(e, e.argumentList.leftParenthesis.offset);
  if (el is ConstructorElement) return _constructorCall(f, el, e.argumentList, e, e.staticType, at);
  if (el is! ExecutableElement) {
    for (final a in e.argumentList.arguments) {
      lowerExpr(f, a.argumentExpression);
    }
    return f.nilConst(at);
  }
  final (args, nodes) = _arguments(f, el, e.argumentList);
  return _call(f, el, null, args, e.staticType, e, at, argList: e.argumentList, slots: nodes);
}

/// `.new(3)`, `.named(3)`.
int _dotShorthandConstructor(FnLower f, DotShorthandConstructorInvocation e) {
  final at = f.pos(e, e.argumentList.leftParenthesis.offset);
  final ctor = e.constructorName.element;
  if (ctor is! ConstructorElement) {
    for (final a in e.argumentList.arguments) {
      lowerExpr(f, a.argumentExpression);
    }
    return f.nilConst(at);
  }
  return _constructorCall(f, ctor, e.argumentList, e, e.staticType, at);
}

int _functionInvocation(FnLower f, FunctionExpressionInvocation e) {
  final fn = lowerExpr(f, e.function);
  // An instance of a class that has a `call` method is called like a function: `r(x)` is `r.call(x)` (a language rule,
  // and the static type names the method). The analyzer gives the method as the element of the invocation; a function
  // value has none (and a dynamic one is not known statically): they stay a call of a value.
  final el = e.element;
  if (el is MethodElement && el.name == 'call') {
    final at = f.pos(e, e.argumentList.leftParenthesis.offset);
    final (args, nodes) = _arguments(f, el, e.argumentList);
    return _call(f, el, fn, args, e.staticType, e, at, argList: e.argumentList, slots: nodes);
  }
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
    // a constructor that does not resolve: what it makes is not known (said), and is taken to come from the
    // arguments, as the result of a method that is not known is (more than the truth, never less)
    final args = <int>[];
    for (final a in e.argumentList.arguments) {
      args.add(lowerExpr(f, a.argumentExpression));
    }
    final d = f.tmp(e.staticType);
    f.emit('unsupported', dsts: [d], args: args, at: at, more: {'reason': 'a constructor that does not resolve: ${e.constructorName.toSource()}'});
    return d;
  }
  return _constructorCall(f, ctor, e.argumentList, e, e.staticType, at);
}

/// The call of a constructor (the same for `new C(...)`, a shorthand, and the
/// `super(...)` of another constructor).
int constructorCall(FnLower f, ConstructorElement ctor, ArgumentList list, AstNode n, DartType? t, Pos at,
        {Map<FormalParameterElement, FormalParameterElement>? passed, Pos? passedAt}) =>
    _constructorCall(f, ctor, list, n, t, at, passed: passed, passedAt: passedAt);

/// [passed]: the parameters of [ctor] that the constructor being lowered gets as super parameters;
/// they fill the places that the arguments of the call leave out.
int _constructorCall(FnLower f, ConstructorElement ctor, ArgumentList list, AstNode n, DartType? t, Pos at,
    {Map<FormalParameterElement, FormalParameterElement>? passed, Pos? passedAt}) {
  final (args, nodes) = _arguments(f, ctor, list);
  if (passed != null && passed.isNotEmpty) {
    final params = orderedParams(ctor);
    for (var i = 0; i < params.length && i < args.length; i++) {
      final mine = passed[params[i].baseElement];
      if (mine != null && nodes[i] == null) args[i] = f.readVar(mine.baseElement, mine.type, passedAt ?? at);
    }
  }
  final d = f.tmp(t);
  final pos = f.unit.callPos(n, list, nodes);
  final sym = symbolOf(ctor);
  final names = _argNames(ctor, args.length);
  final instr = f.emit('call', dsts: [d], args: args, at: pos, more: {
    'callee': {'kind': 'static', 'func': funcId(ctor), 'symbol': sym},
    if (names != null) 'arg_names': names,
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
  // an operator that the code defines is a call of its body
  final m = e.element;
  if (m != null && _hasBodyHere(f, m)) return _call(f, m, l, [r], e.staticType, e, f.pos(e, e.operator.offset));
  final d = f.tmp(e.staticType);
  f.emit('binop', dsts: [d], args: [l, r], at: f.pos(e, e.operator.offset), more: {'operator': e.operator.lexeme});
  return d;
}

/// Whether the method is one that the program has a body for (an operator of
/// the project, not `int.+`).
bool _hasBodyHere(FnLower f, ExecutableElement m) => f.prog.bodies.contains(funcId(m));

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
  final m = e.element;
  if (m != null && _hasBodyHere(f, m)) return _call(f, m, v, const [], e.staticType, e, f.pos(e));
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
    // `x ??= v`: v is evaluated and assigned only when x is null; the value is x either way
    final cur = lowerExpr(f, e.leftHandSide);
    final result = f.tmp(e.staticType);
    f.emit('copy', dsts: [result], args: [cur], at: f.pos(e));
    final nilc = f.nilConst(f.pos(e));
    final isNull = f.local('', null);
    f.out.locals[isNull] = Local('', 'dart:core#bool', 'bool');
    f.emit('binop', dsts: [isNull], args: [cur, nilc], at: f.pos(e), more: {'operator': '=='});
    final assign = f.newBlock();
    final join = f.newBlock();
    f.branch(isNull, assign, join, f.pos(e));
    f.cur = assign;
    final v = lowerExpr(f, e.rightHandSide);
    f.emit('copy', dsts: [result], args: [v], at: f.pos(e));
    lowerAssign(f, e.leftHandSide, v);
    f.jump(join);
    f.cur = join;
    return result;
  } else {
    final cur = lowerExpr(f, e.leftHandSide);
    final r = lowerExpr(f, e.rightHandSide);
    // an operator that the code defines is a call of its body (`a += b` calls `+`)
    final m = e.element;
    if (m != null && _hasBodyHere(f, m)) {
      value = _call(f, m, cur, [r], e.staticType, e, f.pos(e));
    } else {
      final d = f.tmp(e.staticType);
      final lex = e.operator.lexeme;
      f.emit('binop', dsts: [d], args: [cur, r], at: f.pos(e), more: {'operator': lex.substring(0, lex.length - 1)});
      value = d;
    }
  }
  lowerAssign(f, e.leftHandSide, value);
  return value;
}

/// The element that `x++`, `--x` and `x += 1` read: the operand has none of its own.
Element? _readElementOf(Expression x) {
  final p = x.parent;
  if (p is AssignmentExpression && identical(p.leftHandSide, x)) return p.readElement;
  if (p is PostfixExpression && identical(p.operand, x)) return p.readElement;
  if (p is PrefixExpression && identical(p.operand, x)) return p.readElement;
  return null;
}

/// The element that an assignment, or `x++` / `--x`, writes: it is the one the
/// operation says (the operand of `++` has none of its own).
Element? _writeElementOf(Expression x) {
  final p = x.parent;
  if (p is AssignmentExpression && identical(p.leftHandSide, x)) return p.writeElement;
  if (p is PostfixExpression && identical(p.operand, x)) return p.writeElement;
  if (p is PrefixExpression && identical(p.operand, x)) return p.writeElement;
  return null;
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
      final we = _writeElementOf(x) ?? el;
      if (we == null) {
        f.emit('unsupported', at: at, more: {'reason': 'assignment to a name that does not resolve: ${x.name}'});
        return;
      }
      if (_isLocal(we)) {
        f.writeVar(we.baseElement, x.staticType, value, at);
      } else {
        _writeMember(f, we, null, x, value, at);
      }
    case PrefixedIdentifier():
      final pre = x.prefix.element;
      final we = _writeElementOf(x) ?? x.identifier.element;
      if (pre is PrefixElement || pre is InterfaceElement) {
        _writeMember(f, we, null, x, value, f.pos(x, x.identifier.offset));
      } else {
        final base = lowerExpr(f, x.prefix);
        _writeMember(f, we, base, x, value, f.pos(x, x.identifier.offset), dynamicName: x.identifier.name);
      }
    case PropertyAccess():
      final we = _writeElementOf(x) ?? x.propertyName.element;
      final t = x.target;
      final base = t == null ? f.cascade : lowerExpr(f, t);
      _writeMember(f, we, base, x, value, f.pos(x, x.propertyName.offset), dynamicName: x.propertyName.name);
    case IndexExpression():
      final t = x.target;
      final base = t == null ? f.cascade! : lowerExpr(f, t);
      final idx = lowerExpr(f, x.index);
      final w = _writeElementOf(x) ?? x.element;
      if (w is ExecutableElement && _hasBodyHere(f, w)) {
        _call(f, w, base, [idx, value], null, x, at);
        return;
      }
      final isMap = _isMap(x.target?.staticType);
      f.emit('index_store', args: [base, idx, value], at: at, more: {if (isMap) 'keyed': true});
    default:
      f.emit('unsupported', at: at, more: {'reason': 'assignment to ${x.runtimeType}'});
  }
}

bool _isMap(DartType? t) => t is InterfaceType && t.element.name == 'Map';

void _writeMember(FnLower f, Element? el, int? base, AstNode n, int value, Pos at, {String? dynamicName}) {
  if (el == null) {
    // a member of a dynamic value
    if (base != null && dynamicName != null) {
      f.emit('field_store', args: [base, value], at: at, more: {'field': dynamicName});
    } else {
      f.emit('unsupported', at: at, more: {'reason': 'assignment to a name that does not resolve: ${dynamicName ?? n.toSource()}'});
    }
    return;
  }
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
      f.emit('unsupported', at: at, more: {'reason': 'assignment to ${target.runtimeType}'});
  }
}

// --- indexing and collections ---------------------------------------------

int _index(FnLower f, IndexExpression e) {
  final t = e.target;
  final base = t == null ? f.cascade! : lowerExpr(f, t);
  final idx = lowerExpr(f, e.index);
  // an index operator that the code defines is a call of its body (`a[i] += v` reads with `[]`: the element of the
  // index expression itself is the `[]=` that writes)
  final re = _readElementOf(e);
  final m = re is ExecutableElement ? re : e.element;
  if (m != null && _hasBodyHere(f, m)) return _call(f, m, base, [idx], e.staticType, e, f.pos(e));
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
      case IfElement():
        // Both branches may contribute: lower them in sequence, after the test.
        lowerExpr(f, el.expression);
        add(el.thenElement);
        final other = el.elseElement;
        if (other != null) add(other);
      case ForElement():
        _forElement(f, el, add);
      case NullAwareElement():
        // `?x`: the element is there when x is not null; x is what it holds
        final v = lowerExpr(f, el.value);
        final i = f.constant('int', '${n++}', null, f.pos(el));
        f.emit('index_store', args: [d, i, v], at: f.pos(el), more: isMap ? {'keyed': true} : {});
      default:
        f.emit('unsupported', at: f.pos(el), more: {'reason': 'collection element ${el.runtimeType}'});
    }
  }

  for (final el in elements) {
    add(el);
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

int lowerClosure(FnLower f, FunctionExpression fe, {String? name, Object? selfKey}) {
  final id = '${f.out.id}\$${f.nextClosure()}';
  final out = FuncOut(id)
    ..symbol = ''
    ..dep = f.out.dep; // a closure of a dependency is a function of the dependency
  out.pos = f.pos(fe);
  final child = FnLower(f.prog, f.unit, out, parent: f, cells: f.cells, assigned: f.assigned);
  if (selfKey != null) {
    child.selfKey = selfKey;
    child.selfFunc = id;
  }
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

/// `[for (final x in xs) element]`: the element is made once per step of the
/// loop, with the variable bound.
void _forElement(FnLower f, ForElement el, void Function(CollectionElement) add) {
  final parts = el.forLoopParts;
  final isAwait = el.awaitKeyword != null;
  switch (parts) {
    case ForEachPartsWithDeclaration():
      final it = lowerExpr(f, parts.iterable);
      final v = parts.loopVariable.declaredFragment?.element;
      f.forEachLoop({}, it, () => add(el.body), isAwait, (value) {
        if (v != null) f.declare(v.baseElement, parts.loopVariable.name.lexeme, v.type, value, f.pos(parts.loopVariable));
      });
    case ForEachPartsWithIdentifier():
      final it = lowerExpr(f, parts.iterable);
      f.forEachLoop({}, it, () => add(el.body), isAwait, (value) => f.assign(parts.identifier, value));
    case ForEachPartsWithPattern():
      final it = lowerExpr(f, parts.iterable);
      f.forEachLoop({}, it, () => add(el.body), isAwait, (value) => f.bindLoopPattern(parts.pattern, value));
    case ForPartsWithDeclarations():
      for (final v in parts.variables.variables) {
        final init = v.initializer;
        final value = init == null ? null : lowerExpr(f, init);
        final e = v.declaredFragment?.element;
        if (e != null) f.declare(e.baseElement, v.name.lexeme, e.type, value, f.pos(v));
      }
      f.classicFor({}, parts.condition, parts.updaters, () => add(el.body));
    case ForPartsWithExpression():
      final i = parts.initialization;
      if (i != null) lowerExpr(f, i);
      f.classicFor({}, parts.condition, parts.updaters, () => add(el.body));
    default:
      f.emit('unsupported', at: f.pos(el), more: {'reason': 'collection for ${parts.runtimeType}'});
  }
}
