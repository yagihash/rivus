// Lowering of a resolved function body to the neutral IR: a control flow
// graph of blocks with mutable locals and no φ functions (ADR-003). The core
// builds the SSA form.
//
// What is decided here (ADR-037; the spike that tried it is gone from the tree, it is in the history):
//  - a call that may throw, inside a try region, ends its block, with one edge
//    for the exception and one for the continuation;
//  - `finally` is copied once for every way out of the try;
//  - an `await` ends the block with an ordinary edge (suspension is not flow);
//  - `&&`, `||`, `?:`, `??` are control flow, joined in a local;
//  - string interpolation is a chain of concatenations of the constant parts
//    and the expressions;
//  - a variable that a closure captures and anything assigns lives in a cell
//    (alloc, load, store); the other variables are locals.
import 'package:analyzer/dart/ast/ast.dart' hide Block;
import 'package:analyzer/dart/ast/ast.dart' as ast show Block;
import 'package:analyzer/dart/ast/token.dart';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';

import 'irjson.dart';
import 'names.dart';
import 'program.dart';

part 'exprs.dart';
part 'patterns.dart';
part 'switches.dart';

class _Try {
  _Try({this.catches, this.fin});
  final NodeList<CatchClause>? catches;
  final ast.Block? fin;
  Blk? dispatch; // where exceptions of the try body go
  Blk? unwind; // the finally, for an exception that no clause takes
}

class _Jump {
  _Jump(this.labels, this.breakTo, this.continueTo, this.depth, {this.unlabeledBreak = true});
  final Set<String> labels;
  /// An unlabeled `break` leaves the innermost loop or switch: a labeled block is not one (#59).
  final bool unlabeledBreak;
  final Blk breakTo;
  final Blk? continueTo;
  final int depth; // the handlers above this depth run their finally
}

/// What a closure asks of the function around it: a variable, or `this`.
final Object thisKey = Object();

class FnLower {
  FnLower(this.prog, this.unit, this.out, {this.parent, this.cells = const {}, this.assigned = const {}});

  final Program prog;
  final UnitInfo unit;
  final FuncOut out;
  final FnLower? parent;

  /// The elements held in cells (captured, and assigned after their
  /// declaration), decided for the outermost function.
  final Set<Object> cells;
  final Set<Object> assigned;

  final Map<Object, int> vars = {}; // variable (or thisKey) -> local
  final Map<Object, int> cellVars = {}; // variable -> local holding its cell
  final List<Object> captures = []; // what the closure takes from outside
  final Map<Object, int> captureLocal = {};
  // a local function that calls itself by its name: the element of the name, the function, and the places that make the
  // closure again (their arguments are what the closure captures, known when the body is lowered)
  Object? selfKey;
  String? selfFunc;
  final List<Instr> _selfMakes = [];
  final _handlers = <_Try>[];
  final _jumps = <_Jump>[];
  var _pendingLabels = <String>{};
  final caught = <int>[]; // the exceptions of the catch clauses we are in
  int? cascade; // the value a cascade section works on
  int? generated; // the collection a generator (sync*, async*) yields into
  Blk? cur;
  Instr? _lastCall;
  int _closures = 0;

  // --- plumbing -----------------------------------------------------------

  Blk newBlock() {
    final b = Blk(out.blocks.length);
    out.blocks.add(b);
    return b;
  }

  int local(String name, DartType? t) {
    out.locals.add(Local(name, typeId(t), typeClass(t)));
    return out.locals.length - 1;
  }

  int tmp(DartType? t) => local('', t);

  Pos pos(AstNode n, [int? offset]) => unit.pos(n, offset ?? n.offset);

  Instr emit(String op, {List<int> dsts = const [], List<int> args = const [], Pos at = Pos.none, Map<String, Object?> more = const {}}) {
    final i = <String, Object?>{
      'op': op,
      if (dsts.isNotEmpty) 'dsts': dsts,
      if (args.isNotEmpty) 'args': args,
      if (!at.isNone) 'pos': at.toJson(),
      ...more,
    };
    cur?.instrs.add(i);
    return i;
  }

  void jump(Blk to, {String kind = 'normal'}) {
    final c = cur;
    if (c == null) return;
    c.term = {'kind': 'jump'};
    c.succs.add(Edge(to.id, kind));
    cur = null;
  }

  void branch(int cond, Blk t, Blk f, Pos at) {
    final c = cur;
    if (c == null) return;
    c.term = {'kind': 'branch', 'cond': cond, if (!at.isNone) 'cond_pos': at.toJson()};
    c.succs
      ..add(Edge(t.id, 'true'))
      ..add(Edge(f.id, 'false'));
    cur = null;
  }

  Blk? _exceptionTarget(int depth) {
    for (var i = depth - 1; i >= 0; i--) {
      final t = _handlers[i];
      if (t.catches != null && t.catches!.isNotEmpty) return t.dispatch ??= newBlock();
      if (t.fin != null) return t.unwind ??= newBlock();
    }
    return null;
  }

  /// The call just emitted may throw: inside a try region it ends its block.
  void mayThrow() {
    final c = cur;
    if (c == null) return;
    final to = _exceptionTarget(_handlers.length);
    if (to == null) return;
    final next = newBlock();
    c.term = {'kind': 'jump'};
    c.succs
      ..add(Edge(next.id, 'normal'))
      ..add(Edge(to.id, 'exception'));
    cur = next;
  }

  void ret(List<int> values, Pos at) {
    if (generated != null && values.isEmpty) values = [generated!];
    runFinallies(0);
    final c = cur;
    if (c == null) return;
    c.term = {'kind': 'return', if (values.isNotEmpty) 'values': values, if (!at.isNone) 'pos': at.toJson()};
    cur = null;
  }

  void thr(int value, Pos at) {
    final c = cur;
    if (c == null) return;
    c.term = {'kind': 'throw', 'values': [value], if (!at.isNone) 'pos': at.toJson()};
    final to = _exceptionTarget(_handlers.length);
    if (to != null) c.succs.add(Edge(to.id, 'exception'));
    cur = null;
  }

  /// Runs the finally blocks between here and the given depth, innermost
  /// first. The finally is copied for each way out of the try.
  void runFinallies(int downTo) {
    for (var i = _handlers.length - 1; i >= downTo && cur != null; i--) {
      final fin = _handlers[i].fin;
      if (fin == null) continue;
      final saved = List<_Try>.of(_handlers);
      _handlers.removeRange(i, _handlers.length);
      stmt(fin);
      _handlers
        ..clear()
        ..addAll(saved);
    }
  }

  // --- constants ----------------------------------------------------------

  int constant(String kind, String value, DartType? t, Pos at) {
    final d = tmp(t);
    emit('const', dsts: [d], at: at, more: {
      'const': {'kind': kind, if (value.isNotEmpty) 'value': value}
    });
    return d;
  }

  int nilConst(Pos at) => constant('nil', '', null, at);

  // --- entry --------------------------------------------------------------

  /// Lowers a function. [params] are the declared parameters, the receiver
  /// first for a method.
  void lowerBody({
    required List<(Object, String, DartType?)> params,
    required FunctionBody body,
    required Pos at,
    List<Statement> prologue = const [],
    void Function()? preamble,
  }) {
    cur = newBlock();
    for (var i = 0; i < params.length; i++) {
      final (key, name, t) = params[i];
      final l = local(name, t);
      out.params.add(Param(name, typeId(t), typeClass(t), displayType(t)));
      emit('param', dsts: [l], at: at, more: {'index': i});
      bind(key, l);
    }
    preamble?.call();
    if (body.isGenerator) {
      // What a generator yields is its result: a collection of the values.
      final g = tmp(null);
      out.locals[g] = Local('', '', 'container');
      emit('alloc', dsts: [g], at: at, more: {'type': 'generated', 'class': 'container'});
      generated = g;
    }
    if (body is BlockFunctionBody) {
      stmt(body.block);
      if (cur != null) ret([], Pos.none);
    } else if (body is ExpressionFunctionBody) {
      final v = expr(body.expression);
      ret([v], pos(body.expression));
    } else {
      if (cur != null) ret([], Pos.none);
    }
    finishCaptures();
  }

  /// Lowers a function that has no parameters and no body to read: the code
  /// is made by [body] (the initializer of a variable).
  void lowerInit({required Pos at, required void Function() body}) {
    cur = newBlock();
    body();
    if (cur != null) ret([], Pos.none);
    finishCaptures();
  }

  /// A parameter or a variable that is held in a cell lives behind a pointer.
  void bind(Object key, int l) {
    if (cells.contains(key) && assigned.contains(key)) {
      final c = tmp(null);
      out.locals[c] = Local('', '', 'pointer');
      emit('alloc', dsts: [c], more: {'type': 'cell', 'class': out.locals[l].cls});
      emit('store', args: [c, l]);
      cellVars[key] = c;
    } else {
      vars[key] = l;
    }
  }

  /// The captured values of a closure are parameters after the declared ones.
  void finishCaptures() {
    final first = out.blocks.first;
    final at = <Instr>[];
    final declared = out.params.length;
    for (var i = 0; i < captures.length; i++) {
      final key = captures[i];
      final l = captureLocal[key]!;
      final isCell = cellVars.containsKey(key);
      final cl = out.locals[l];
      out.params.add(Param(cl.name, cl.type, cl.cls, '', captured: true));
      at.add({'op': 'param', 'dsts': [l], 'index': declared + i});
      if (isCell) {
        // the captured value is the cell itself
      }
    }
    first.instrs.insertAll(0, at);
    // the closure that a local function makes of itself takes what it has captured (the parameters after the declared ones)
    for (final m in _selfMakes) {
      final args = [for (final key in captures) captureLocal[key]!];
      if (args.isNotEmpty) m['args'] = args;
    }
  }

  // --- variables ----------------------------------------------------------

  /// The local that holds the value (or the cell) of a variable, asking the
  /// function around when it is not ours.
  int? _lookup(Object key, DartType? t, {required bool wantCell}) {
    if (vars.containsKey(key) && !wantCell) return vars[key];
    if (selfKey != null && key == selfKey && !wantCell && !vars.containsKey(key)) {
      // a closure inside the local function takes the function itself (the call of its own name from a closure in its body)
      final d = tmp(t);
      _selfMakes.add(emit('make_closure', dsts: [d], more: {'func': selfFunc}));
      return d;
    }
    if (cellVars.containsKey(key)) return cellVars[key];
    if (vars.containsKey(key)) return vars[key];
    final p = parent;
    if (p == null) return null;
    // Ask the function around: it holds the value, or the cell.
    final outer = p._lookup(key, t, wantCell: wantCell);
    if (outer == null) return null;
    final isCell = p.cellVars.containsKey(key) || (p.captureLocal.containsKey(key) && p.cellVars.containsKey(key));
    if (!captureLocal.containsKey(key)) {
      final l = local('', t);
      captureLocal[key] = l;
      captures.add(key);
      if (isCell) {
        out.locals[l] = Local('', '', 'pointer');
        cellVars[key] = l;
      } else {
        vars[key] = l;
      }
    }
    return captureLocal[key];
  }

  int readVar(Object key, DartType? t, Pos at) {
    if (selfKey != null && key == selfKey && !vars.containsKey(key)) {
      // the name of a local function, inside its own body (a recursive call): the function itself
      final d = tmp(t);
      _selfMakes.add(emit('make_closure', dsts: [d], at: at, more: {'func': selfFunc}));
      return d;
    }
    final l = _lookup(key, t, wantCell: false);
    if (l == null) {
      // a variable that the lowering does not know: a mistake of the extractor, said
      emit('unsupported', at: at, more: {'reason': 'a variable that the extractor does not know ($key)'});
      return nilConst(at);
    }
    if (cellVars.containsKey(key)) {
      final d = tmp(t);
      emit('load', dsts: [d], args: [cellVars[key]!], at: at);
      return d;
    }
    return l;
  }

  void writeVar(Object key, DartType? t, int value, Pos at) {
    final l = _lookup(key, t, wantCell: false);
    if (l == null) return;
    if (cellVars.containsKey(key)) {
      emit('store', args: [cellVars[key]!, value], at: at);
    } else {
      emit('copy', dsts: [l], args: [value], at: at);
    }
  }

  void declare(Object key, String name, DartType? t, int? init, Pos at) {
    final l = local(name, t);
    if (init != null) {
      emit('copy', dsts: [l], args: [init], at: at);
    } else {
      emit('const', dsts: [l], at: at, more: {
        'const': {'kind': 'nil'}
      });
    }
    bind(key, l);
  }

  // --- statements ---------------------------------------------------------

  void stmt(Statement s) {
    if (cur == null) return;
    switch (s) {
      case ast.Block():
        for (final x in s.statements) {
          stmt(x);
        }
      case ExpressionStatement():
        expr(s.expression);
      case VariableDeclarationStatement():
        for (final v in s.variables.variables) {
          final init = v.initializer;
          final value = init == null ? null : expr(init);
          final el = v.declaredFragment?.element;
          if (el == null) continue;
          declare(el.baseElement, v.name.lexeme, el.type, value, pos(v));
        }
      case IfStatement():
        _if(s);
      case ForStatement():
        _for(s);
      case WhileStatement():
        _while(s.condition, s.body);
      case DoStatement():
        _do(s);
      case ReturnStatement():
        final e = s.expression;
        final v = e == null ? null : expr(e);
        ret(v == null ? [] : [v], pos(s));
      case BreakStatement():
        _breakOrContinue(s.label?.name.lexeme, isBreak: true);
      case ContinueStatement():
        _breakOrContinue(s.label?.name.lexeme, isBreak: false);
      case LabeledStatement():
        for (final l in s.labels) {
          _pendingLabels.add(l.name.lexeme);
        }
        final inner = s.statement;
        if (inner is ForStatement || inner is WhileStatement || inner is DoStatement || inner is SwitchStatement) {
          stmt(inner);
          _pendingLabels = {};
        } else {
          // a labeled block (`done: { ... break done; ... }`): the break goes to what follows it
          final labels = _takeLabels();
          final after = newBlock();
          _jumps.add(_Jump(labels, after, null, _handlers.length, unlabeledBreak: false));
          stmt(inner);
          _jumps.removeLast();
          jump(after);
          cur = _reached(after) ? after : null;
        }
      case TryStatement():
        _try(s);
      case SwitchStatement():
        _switch(s);
      case YieldStatement():
        final v = expr(s.expression);
        if (s.star != null) {
          // `yield* xs;` yields every element of xs, one by one
          final it = tmp(null);
          emit('call', dsts: [it], args: [v], at: pos(s), more: {
            'callee': {'kind': 'static', 'symbol': 'iter.start'},
          });
          final ok = local('', null);
          final k = local('', null);
          final e = local('', null);
          out.locals[ok] = Local('', 'dart:core#bool', 'bool');
          emit('call', dsts: [ok, k, e], args: [it], at: pos(s), more: {
            'callee': {'kind': 'static', 'symbol': 'iter.next'},
          });
          _yield(e, s);
        } else {
          _yield(v, s);
        }
      case PatternVariableDeclarationStatement():
        final d = s.declaration;
        final v = expr(d.expression);
        final miss = newBlock();
        bindPattern(d.pattern, v, miss);
        // a pattern that does not match throws
        final matched = newBlock();
        jump(matched);
        cur = miss;
        thr(constant('string', 'pattern did not match', null, pos(s)), pos(s));
        cur = matched;
      case EmptyStatement():
        break;
      case AssertStatement():
        // the condition and the message are code that runs (in a checked mode): a call in
        // them can reach a sink, or change what the program holds
        expr(s.condition);
        final m = s.message;
        if (m != null) expr(m);
      case FunctionDeclarationStatement():
        final fd = s.functionDeclaration;
        final el = fd.declaredFragment?.element;
        final clo = closure(fd.functionExpression, name: fd.name.lexeme, selfKey: el?.baseElement);
        if (el != null) declare(el.baseElement, fd.name.lexeme, el.type, clo, pos(fd));
      default:
        emit('unsupported', at: pos(s), more: {'reason': 'statement ${s.runtimeType}'});
    }
  }

  void _yield(int v, YieldStatement s) {
    // The value goes into the result of the generator, where whoever reads
    // the stream or the iterable finds it; then the function is suspended.
    final g = generated;
    if (g != null) {
      final i = constant('int', '0', null, pos(s));
      emit('index_store', args: [g, i, v], at: pos(s));
    }
    final next = newBlock();
    jump(next);
    cur = next;
  }

  Set<String> _takeLabels() {
    final l = _pendingLabels;
    _pendingLabels = {};
    return l;
  }

  void _if(IfStatement s) {
    final thenB = newBlock();
    final elseB = newBlock();
    final join = newBlock();
    final cc = s.caseClause;
    if (cc != null) {
      // if (value case pattern when guard)
      final v = expr(s.expression);
      bindGuarded(cc.guardedPattern, v, elseB);
      jump(thenB);
    } else {
      cond(s.expression, thenB, elseB);
    }
    cur = thenB;
    stmt(s.thenStatement);
    jump(join);
    cur = elseB;
    final e = s.elseStatement;
    if (e != null) stmt(e);
    jump(join);
    cur = _reached(join) ? join : null;
  }

  bool _reached(Blk b) => out.blocks.any((x) => x.succs.any((e) => e.to == b.id));

  void _while(Expression c, Statement body) {
    final labels = _takeLabels();
    final head = newBlock();
    final bodyB = newBlock();
    final after = newBlock();
    jump(head);
    cur = head;
    cond(c, bodyB, after);
    cur = bodyB;
    _jumps.add(_Jump(labels, after, head, _handlers.length));
    stmt(body);
    _jumps.removeLast();
    jump(head, kind: 'back');
    cur = _reached(after) ? after : null;
  }

  void _do(DoStatement s) {
    final labels = _takeLabels();
    final bodyB = newBlock();
    final test = newBlock();
    final after = newBlock();
    jump(bodyB);
    cur = bodyB;
    _jumps.add(_Jump(labels, after, test, _handlers.length));
    stmt(s.body);
    _jumps.removeLast();
    jump(test);
    cur = test;
    cond(s.condition, bodyB, after);
    cur = _reached(after) ? after : null;
  }

  void _for(ForStatement s) {
    final labels = _takeLabels();
    final parts = s.forLoopParts;
    switch (parts) {
      case ForPartsWithDeclarations():
        for (final v in parts.variables.variables) {
          final init = v.initializer;
          final value = init == null ? null : expr(init);
          final el = v.declaredFragment?.element;
          if (el != null) declare(el.baseElement, v.name.lexeme, el.type, value, pos(v));
        }
        classicFor(labels, parts.condition, parts.updaters, () => stmt(s.body));
      case ForPartsWithExpression():
        final i = parts.initialization;
        if (i != null) expr(i);
        classicFor(labels, parts.condition, parts.updaters, () => stmt(s.body));
      case ForEachPartsWithDeclaration():
        final it = expr(parts.iterable);
        final el = parts.loopVariable.declaredFragment?.element;
        forEachLoop(labels, it, () => stmt(s.body), s.awaitKeyword != null, (value) {
          if (el != null) declare(el.baseElement, parts.loopVariable.name.lexeme, el.type, value, pos(parts.loopVariable));
        });
      case ForEachPartsWithIdentifier():
        final it = expr(parts.iterable);
        forEachLoop(labels, it, () => stmt(s.body), s.awaitKeyword != null, (value) => assign(parts.identifier, value));
      case ForEachPartsWithPattern():
        final it = expr(parts.iterable);
        forEachLoop(labels, it, () => stmt(s.body), s.awaitKeyword != null, (value) => bindLoopPattern(parts.pattern, value));
      default:
        emit('unsupported', at: pos(s), more: {'reason': 'for loop ${parts.runtimeType}'});
    }
  }

  void bindLoopPattern(DartPattern pattern, int value) {
    final miss = newBlock();
    bindPattern(pattern, value, miss);
    final matched = newBlock();
    jump(matched);
    cur = miss;
    jump(matched);
    cur = matched;
  }

  void classicFor(Set<String> labels, Expression? c, NodeList<Expression> updaters, void Function() body) {
    final head = newBlock();
    final bodyB = newBlock();
    final update = newBlock();
    final after = newBlock();
    jump(head);
    cur = head;
    if (c != null) {
      cond(c, bodyB, after);
    } else {
      jump(bodyB);
    }
    cur = bodyB;
    _jumps.add(_Jump(labels, after, update, _handlers.length));
    body();
    _jumps.removeLast();
    jump(update);
    cur = update;
    for (final u in updaters) {
      expr(u);
    }
    jump(head, kind: 'back');
    cur = _reached(after) ? after : null;
  }

  void forEachLoop(Set<String> labels, int iterable, void Function() body, bool isAwait, void Function(int) bind) {
    final it = tmp(null);
    emit('call', dsts: [it], args: [iterable], more: {
      'callee': {'kind': 'static', 'symbol': 'iter.start'},
    });
    final head = newBlock();
    final bodyB = newBlock();
    final after = newBlock();
    jump(head);
    cur = head;
    final ok = local('', null);
    final k = local('', null);
    final v = local('', null);
    out.locals[ok] = Local('', 'dart:core#bool', 'bool');
    emit('call', dsts: [ok, k, v], args: [it], more: {
      'callee': {'kind': 'static', 'symbol': 'iter.next'},
      if (isAwait) 'await': true,
    });
    branch(ok, bodyB, after, Pos.none);
    cur = bodyB;
    _jumps.add(_Jump(labels, after, head, _handlers.length));
    bind(v);
    body();
    _jumps.removeLast();
    jump(head, kind: 'back');
    cur = _reached(after) ? after : null;
  }

  final List<(Map<String, Blk>, int)> _caseLabels = [];

  void _breakOrContinue(String? label, {required bool isBreak}) {
    if (label != null && !isBreak) {
      for (var i = _caseLabels.length - 1; i >= 0; i--) {
        final (labels, depth) = _caseLabels[i];
        final to = labels[label];
        if (to != null) {
          runFinallies(depth);
          jump(to);
          return;
        }
      }
    }
    _Jump? target;
    for (var i = _jumps.length - 1; i >= 0; i--) {
      final j = _jumps[i];
      if (label == null ? (isBreak ? j.unlabeledBreak : j.continueTo != null) : j.labels.contains(label)) {
        target = j;
        break;
      }
    }
    if (target == null) {
      // the target is not one that is known: what follows is not where the flow goes
      emit('unsupported', more: {'reason': '${isBreak ? 'break' : 'continue'} ${label ?? ''} to a place that is not known'});
      cur = null;
      return;
    }
    runFinallies(target.depth);
    final to = isBreak ? target.breakTo : (target.continueTo ?? target.breakTo);
    jump(to);
  }

  void _switch(SwitchStatement s) {
    final labels = _takeLabels();
    final subject = expr(s.expression);
    final after = newBlock();
    _jumps.add(_Jump(labels, after, null, _handlers.length));
    final bodies = [for (final _ in s.members) newBlock()];
    // `continue name;` goes to the case that has the label
    final caseLabels = <String, Blk>{};
    for (var i = 0; i < s.members.length; i++) {
      for (final lb in s.members[i].labels) {
        caseLabels[lb.name.lexeme] = bodies[i];
      }
    }
    _caseLabels.add((caseLabels, _handlers.length));
    Blk? defaultBody;
    for (var i = 0; i < s.members.length; i++) {
      final m = s.members[i];
      if (m is SwitchDefault) defaultBody = bodies[i];
    }
    for (var i = 0; i < s.members.length; i++) {
      final m = s.members[i];
      if (m is SwitchCase) {
        final test = expr(m.expression);
        final eq = local('', null);
        out.locals[eq] = Local('', 'dart:core#bool', 'bool');
        emit('binop', dsts: [eq], args: [subject, test], at: pos(m), more: {'operator': '=='});
        final miss = newBlock();
        branch(eq, bodies[i], miss, pos(m));
        cur = miss;
      } else if (m is SwitchPatternCase) {
        final miss = newBlock();
        bindGuarded(m.guardedPattern, subject, miss);
        jump(bodies[i]);
        cur = miss;
      }
    }
    jump(defaultBody ?? after);
    for (var i = 0; i < s.members.length; i++) {
      cur = bodies[i];
      for (final st in s.members[i].statements) {
        stmt(st);
      }
      // A case that is empty goes on to the next one (`case 1: case 2: ...`); one that is not ends
      // at the end of the switch (Dart 3 does not fall through, and before it the analyzer rejected it).
      jump(i + 1 < bodies.length && s.members[i].statements.isEmpty ? bodies[i + 1] : after);
    }
    _caseLabels.removeLast();
    _jumps.removeLast();
    cur = _reached(after) ? after : null;
  }

  void _try(TryStatement s) {
    final t = _Try(catches: s.catchClauses, fin: s.finallyBlock);
    final after = newBlock();
    _handlers.add(t);
    stmt(s.body);
    _handlers.removeLast();
    // normal exit of the body
    if (cur != null) {
      final fin = s.finallyBlock;
      if (fin != null) {
        final saved = List<_Try>.of(_handlers);
        stmt(fin);
        _handlers
          ..clear()
          ..addAll(saved);
      }
      jump(after);
    }
    // an exception in the body: the catch clauses
    if (t.dispatch != null) {
      cur = t.dispatch;
      final exc = tmp(null);
      emit('exception_value', dsts: [exc]);
      caught.add(exc);
      // a throw in a clause runs the finally: the same handler for every clause, and
      // the block it makes is the one that the try makes at its end
      final inClause = _Try(fin: s.finallyBlock);
      _handlers.add(inClause);
      for (final c in s.catchClauses) {
        final body = newBlock();
        final next = newBlock();
        final type = c.exceptionType;
        if (type == null) {
          jump(body);
        } else {
          final test = local('', null);
          out.locals[test] = Local('', 'dart:core#bool', 'bool');
          final tn = constant('string', type.toSource(), null, pos(c));
          emit('binop', dsts: [test], args: [exc, tn], at: pos(c), more: {'operator': 'is'});
          branch(test, body, next, pos(c));
        }
        cur = body;
        final ep = c.exceptionParameter;
        final el = ep?.declaredFragment?.element;
        if (ep != null && el != null) declare(el.baseElement, ep.name.lexeme, el.type, exc, pos(c));
        final sp = c.stackTraceParameter;
        final sel = sp?.declaredFragment?.element;
        if (sp != null && sel != null) declare(sel.baseElement, sp.name.lexeme, sel.type, null, pos(c));
        stmt(c.body);
        _handlers.removeLast();
        if (cur != null) {
          final fin = s.finallyBlock;
          if (fin != null) {
            final saved = List<_Try>.of(_handlers);
            stmt(fin);
            _handlers
              ..clear()
              ..addAll(saved);
          }
          jump(after);
        }
        _handlers.add(inClause);
        cur = type == null ? null : next;
        if (type == null) break;
      }
      _handlers.removeLast();
      caught.removeLast();
      if (inClause.unwind != null) t.unwind ??= inClause.unwind;
      // no clause took the exception: it goes on, after the finally
      if (cur != null) {
        final fin = s.finallyBlock;
        if (fin != null) stmt(fin);
        thr(exc, Pos.none);
      }
    }
    // an exception that no clause covers, with only a finally
    if (t.unwind != null) {
      cur = t.unwind;
      final exc = tmp(null);
      emit('exception_value', dsts: [exc]);
      final fin = s.finallyBlock;
      if (fin != null) stmt(fin);
      thr(exc, Pos.none);
    }
    cur = _reached(after) ? after : null;
  }

  // --- conditions ---------------------------------------------------------

  /// Branches to [t] or [f] on the value of the condition.
  void cond(Expression e, Blk t, Blk f) {
    var x = e;
    while (x is ParenthesizedExpression) {
      x = x.expression;
    }
    if (x is BinaryExpression && x.operator.type == TokenType.AMPERSAND_AMPERSAND) {
      final mid = newBlock();
      cond(x.leftOperand, mid, f);
      cur = mid;
      cond(x.rightOperand, t, f);
      return;
    }
    if (x is BinaryExpression && x.operator.type == TokenType.BAR_BAR) {
      final mid = newBlock();
      cond(x.leftOperand, t, mid);
      cur = mid;
      cond(x.rightOperand, t, f);
      return;
    }
    if (x is PrefixExpression && x.operator.type == TokenType.BANG) {
      cond(x.operand, f, t);
      return;
    }
    final v = expr(x);
    branch(v, t, f, pos(x));
  }

  // --- expressions --------------------------------------------------------

  /// Lowers an expression and returns the local that holds its value.
  int expr(Expression e) => lowerExpr(this, e);

  void assign(Expression target, int value) => lowerAssign(this, target, value);

  /// A closure: its own function, made here.
  int closure(FunctionExpression fe, {String? name, Object? selfKey}) => lowerClosure(this, fe, name: name, selfKey: selfKey);

  int nextClosure() => ++_closures;
}
