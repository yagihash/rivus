// Throwaway spike: build a control flow graph from the resolved AST of
// package:analyzer. The aim is to find out what is hard, not to be complete
// (docs/ARCHITECTURE.md, Dart spike). Every decision that the real extractor
// will have to take is marked "DECISION" and collected in the README.
import 'package:analyzer/dart/ast/ast.dart' hide Block;
import 'package:analyzer/dart/ast/ast.dart' as ast show Block;
import 'package:analyzer/dart/ast/token.dart';

enum EdgeKind {
  normal,
  ifTrue,
  ifFalse,
  back, // the back edge of a loop
  brk,
  cont,
  ret,
  exception, // from something that may throw to a handler
  awaitResume, // await, yield, await for: the function is suspended here
  caseMatch,
  caseFail,
  catchMatch,
}

class Edge {
  Edge(this.to, this.kind, [this.note]);
  final Block to;
  final EdgeKind kind;
  final String? note;
}

class Block {
  Block(this.id, this.label);
  final int id;
  final String label;
  final List<String> items = [];
  final List<Edge> out = [];
}

class Cfg {
  Cfg(this.name);
  final String name;
  final List<Block> blocks = [];
  late Block entry;
  late Block exit; // normal return and uncaught exceptions both end here

  Block newBlock(String label) {
    final b = Block(blocks.length, label);
    blocks.add(b);
    return b;
  }

  Iterable<Edge> get edges sync* {
    for (final b in blocks) {
      yield* b.out;
    }
  }

  Map<EdgeKind, int> get edgeCounts {
    final m = <EdgeKind, int>{};
    for (final e in edges) {
      m[e.kind] = (m[e.kind] ?? 0) + 1;
    }
    return m;
  }

  /// Blocks reachable from the entry.
  Set<Block> get reachable {
    final seen = <Block>{};
    void visit(Block b) {
      if (!seen.add(b)) return;
      for (final e in b.out) {
        visit(e.to);
      }
    }

    visit(entry);
    return seen;
  }

  /// Blocks from which the exit can be reached.
  Set<Block> get reachExit {
    final preds = <Block, List<Block>>{};
    for (final b in blocks) {
      for (final e in b.out) {
        (preds[e.to] ??= []).add(b);
      }
    }
    final seen = <Block>{};
    void visit(Block b) {
      if (!seen.add(b)) return;
      for (final p in preds[b] ?? const <Block>[]) {
        visit(p);
      }
    }

    visit(exit);
    return seen;
  }

  String toDot() {
    final sb = StringBuffer('digraph "$name" {\n  node [shape=box,fontname=monospace];\n');
    for (final b in blocks) {
      final text = [b.label, ...b.items].join('\\n').replaceAll('"', '\\"');
      sb.writeln('  b${b.id} [label="$text"];');
    }
    for (final b in blocks) {
      for (final e in b.out) {
        final label = e.kind == EdgeKind.normal ? '' : e.kind.name + (e.note != null ? ' ${e.note}' : '');
        sb.writeln('  b${b.id} -> b${e.to.id} [label="$label"];');
      }
    }
    sb.writeln('}');
    return sb.toString();
  }
}

class _Jump {
  _Jump(this.labels, this.breakTo, this.continueTo, this.handlerDepth);
  final Set<String> labels;
  final Block breakTo;
  final Block? continueTo; // null for a switch
  final int handlerDepth; // finally blocks above this depth run when jumping
}

class _Try {
  _Try({required this.hasCatch, this.fin});
  final bool hasCatch;
  final ast.Block? fin;
  Block? dispatch; // where exceptions of the try body go (catch clauses)
  Block? unwind; // finally, then on to the outer handler
}

class CfgBuilder {
  CfgBuilder(String name) : cfg = Cfg(name);

  final Cfg cfg;
  Block? cur; // null: the code here cannot be reached
  final _jumps = <_Jump>[];
  final _handlers = <_Try>[];
  var _pendingLabels = <String>{};

  /// Builds the graph of a function body.
  Cfg build(FunctionBody body) {
    cfg.entry = cfg.newBlock('entry');
    cfg.exit = cfg.newBlock('exit');
    cur = cfg.entry;
    if (body is BlockFunctionBody) {
      _stmt(body.block);
    } else if (body is ExpressionFunctionBody) {
      _expr(body.expression);
      _jump(cfg.exit, EdgeKind.ret);
    }
    if (cur != null) _jump(cfg.exit, EdgeKind.normal);
    return cfg;
  }

  // --- plumbing ------------------------------------------------------------

  void _edge(Block from, Block to, EdgeKind kind, [String? note]) => from.out.add(Edge(to, kind, note));

  /// Ends the current block with an edge. The code after it is unreachable
  /// unless something starts a new block.
  void _jump(Block to, EdgeKind kind, [String? note]) {
    final c = cur;
    if (c == null) return;
    _edge(c, to, kind, note);
    cur = null;
  }

  void _item(String s) {
    final c = cur;
    if (c == null) return;
    c.items.add(s.length > 70 ? '${s.substring(0, 67)}...' : s);
  }

  /// An exception can leave the current block: to the nearest handler.
  /// DECISION 1: with no handler in the function nothing is drawn. An
  /// exception that leaves the function is not a flow inside it, and drawing
  /// it from every call would bury the edges that matter (those into catch
  /// bodies).
  /// DECISION 2: inside a try region, a call that may throw ends its block:
  /// one edge for the exception, one for the normal continuation, as the
  /// terminator of a MIR call does. An exception edge per block is wrong as
  /// soon as a block holds two calls, or code that is not covered by the
  /// handler (a finally copy that follows).
  void _mayThrow() {
    final c = cur;
    if (c == null) return;
    final to = _exceptionTarget(_handlers.length);
    if (to == cfg.exit) return;
    _edge(c, to, EdgeKind.exception);
    final next = cfg.newBlock('after call');
    _edge(c, next, EdgeKind.normal);
    cur = next;
  }

  Block _exceptionTarget(int depth) {
    for (var i = depth - 1; i >= 0; i--) {
      final t = _handlers[i];
      if (t.hasCatch) return t.dispatch ??= cfg.newBlock('catch.dispatch');
      if (t.fin != null) return t.unwind ??= cfg.newBlock('unwind');
    }
    return cfg.exit;
  }

  /// Runs the finally blocks between the current position and the given
  /// handler depth, innermost first. DECISION: the finally block is copied
  /// once per way of leaving the try (the other choice is one copy that
  /// dispatches on how it was entered). Copies keep the graph free of
  /// infeasible paths, at the price of more blocks.
  void _runFinallies(int downTo) {
    for (var i = _handlers.length - 1; i >= downTo && cur != null; i--) {
      final fin = _handlers[i].fin;
      if (fin == null) continue;
      final saved = List<_Try>.of(_handlers);
      _handlers.removeRange(i, _handlers.length);
      _item('finally (copy)');
      _stmt(fin);
      _handlers
        ..clear()
        ..addAll(saved);
    }
  }

  // --- statements ----------------------------------------------------------

  void _stmt(Statement s) {
    if (cur == null) return; // dead code
    switch (s) {
      case ast.Block():
        for (final x in s.statements) {
          _stmt(x);
        }
      case ExpressionStatement():
        _expr(s.expression);
      case VariableDeclarationStatement():
        for (final v in s.variables.variables) {
          final init = v.initializer;
          if (init != null) _expr(init);
          _item('${v.name.lexeme} = ...');
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
        if (e != null) _expr(e);
        _item('return');
        _runFinallies(0);
        _jump(cfg.exit, EdgeKind.ret);
      case BreakStatement():
        _breakOrContinue(s.label?.name.lexeme, isBreak: true);
      case ContinueStatement():
        _breakOrContinue(s.label?.name.lexeme, isBreak: false);
      case LabeledStatement():
        for (final l in s.labels) {
          _pendingLabels.add(l.name.lexeme);
        }
        _stmt(s.statement);
        _pendingLabels = {};
      case TryStatement():
        _try(s);
      case SwitchStatement():
        _switch(s);
      case YieldStatement():
        _expr(s.expression);
        _suspend('yield');
      case EmptyStatement():
        break;
      case FunctionDeclarationStatement():
        // DECISION: a local function is its own graph; the enclosing graph
        // only sees the closure value being created.
        _item('local function ${s.functionDeclaration.name.lexeme}');
      case AssertStatement():
        break;
      default:
        _item('<unhandled ${s.runtimeType}>');
    }
  }

  Set<String> _takeLabels() {
    final l = _pendingLabels;
    _pendingLabels = {};
    return l;
  }

  void _if(IfStatement s) {
    final thenB = cfg.newBlock('then');
    final elseB = cfg.newBlock('else');
    final join = cfg.newBlock('if.join');
    _cond(s.expression, thenB, elseB);
    cur = thenB;
    _stmt(s.thenStatement);
    _jump(join, EdgeKind.normal);
    cur = elseB;
    final e = s.elseStatement;
    if (e != null) _stmt(e);
    _jump(join, EdgeKind.normal);
    cur = join;
    _dropIfDead(join);
  }

  /// A join block nobody jumps to is dead: the code after it cannot run.
  void _dropIfDead(Block join) {
    final reached = cfg.blocks.any((b) => b.out.any((e) => e.to == join));
    if (!reached) cur = null;
  }

  void _for(ForStatement s) {
    final labels = _takeLabels();
    final parts = s.forLoopParts;
    final head = cfg.newBlock('for.head');
    final body = cfg.newBlock('for.body');
    final after = cfg.newBlock('for.after');
    late final Block update = cfg.newBlock('for.update'); // not for for-in
    switch (parts) {
      case ForPartsWithDeclarations():
        for (final v in parts.variables.variables) {
          final init = v.initializer;
          if (init != null) _expr(init);
        }
        _jump(head, EdgeKind.normal);
        cur = head;
        final c = parts.condition;
        if (c != null) {
          _cond(c, body, after);
        } else {
          _jump(body, EdgeKind.normal);
        }
        _loopBody(labels, after, update, s.body, body);
        cur = update;
        for (final u in parts.updaters) {
          _expr(u);
        }
        _jump(head, EdgeKind.back);
      case ForPartsWithExpression():
        final i = parts.initialization;
        if (i != null) _expr(i);
        _jump(head, EdgeKind.normal);
        cur = head;
        final c = parts.condition;
        if (c != null) {
          _cond(c, body, after);
        } else {
          _jump(body, EdgeKind.normal);
        }
        _loopBody(labels, after, update, s.body, body);
        cur = update;
        for (final u in parts.updaters) {
          _expr(u);
        }
        _jump(head, EdgeKind.back);
      case ForPartsWithPattern():
        // DECISION: a pattern in a for header is out of the spike's scope.
        _item('<for with a pattern>');
        _jump(head, EdgeKind.normal);
        cur = head;
        final c = parts.condition;
        if (c != null) {
          _cond(c, body, after);
        } else {
          _jump(body, EdgeKind.normal);
        }
        _loopBody(labels, after, update, s.body, body);
        cur = update;
        for (final u in parts.updaters) {
          _expr(u);
        }
        _jump(head, EdgeKind.back);
      case ForEachParts():
        // DECISION: for-in is "call iterator; loop: has next? element".
        // The iterator protocol is left out of the graph, the loop shape is not.
        _expr(parts.iterable);
        _jump(head, EdgeKind.normal);
        cur = head;
        _item(s.awaitKeyword != null ? 'await for: next?' : 'for-in: next?');
        final c = cur;
        if (c != null) {
          _edge(c, body, s.awaitKeyword != null ? EdgeKind.awaitResume : EdgeKind.ifTrue);
          _edge(c, after, EdgeKind.ifFalse);
          if (s.awaitKeyword != null) _edge(c, _exceptionTarget(_handlers.length), EdgeKind.exception);
          cur = null;
        }
        _loopBody(labels, after, head, s.body, body, backTo: head);
        cur = after;
        return;
    }
    cur = after;
    _dropIfDead(after);
  }

  /// Visits a loop body with its break and continue targets.
  void _loopBody(Set<String> labels, Block after, Block cont, Statement body, Block bodyBlock, {Block? backTo}) {
    _jumps.add(_Jump(labels, after, cont, _handlers.length));
    cur = bodyBlock;
    _stmt(body);
    _jumps.removeLast();
    if (backTo != null) {
      _jump(backTo, EdgeKind.back);
    } else {
      _jump(cont, EdgeKind.normal);
    }
  }

  void _while(Expression condition, Statement body) {
    final labels = _takeLabels();
    final head = cfg.newBlock('while.head');
    final bodyB = cfg.newBlock('while.body');
    final after = cfg.newBlock('while.after');
    _jump(head, EdgeKind.normal);
    cur = head;
    _cond(condition, bodyB, after);
    _loopBody(labels, after, head, body, bodyB, backTo: head);
    cur = after;
    _dropIfDead(after);
  }

  void _do(DoStatement s) {
    final labels = _takeLabels();
    final bodyB = cfg.newBlock('do.body');
    final test = cfg.newBlock('do.test');
    final after = cfg.newBlock('do.after');
    _jump(bodyB, EdgeKind.normal);
    _jumps.add(_Jump(labels, after, test, _handlers.length));
    cur = bodyB;
    _stmt(s.body);
    _jumps.removeLast();
    _jump(test, EdgeKind.normal);
    cur = test;
    _cond(s.condition, bodyB, after, backEdgeTrue: true);
    cur = after;
    _dropIfDead(after);
  }

  void _breakOrContinue(String? label, {required bool isBreak}) {
    _Jump? target;
    for (var i = _jumps.length - 1; i >= 0; i--) {
      final j = _jumps[i];
      final matches = label == null ? (isBreak || j.continueTo != null) : j.labels.contains(label);
      if (matches) {
        target = j;
        break;
      }
    }
    if (target == null) {
      _item('<jump target not found>');
      cur = null;
      return;
    }
    _item(isBreak ? 'break${label != null ? ' $label' : ''}' : 'continue${label != null ? ' $label' : ''}');
    _runFinallies(target.handlerDepth);
    _jump(isBreak ? target.breakTo : target.continueTo!, isBreak ? EdgeKind.brk : EdgeKind.cont);
  }

  void _try(TryStatement t) {
    final hasCatch = t.catchClauses.isNotEmpty;
    final fin = t.finallyBlock;
    final ctx = _Try(hasCatch: hasCatch, fin: fin);
    final join = cfg.newBlock('try.join');
    if (hasCatch) ctx.dispatch = cfg.newBlock('catch.dispatch');
    if (fin != null) ctx.unwind = cfg.newBlock('unwind');

    // The body.
    _handlers.add(ctx);
    final bodyStart = cfg.newBlock('try.body');
    _jump(bodyStart, EdgeKind.normal);
    cur = bodyStart;
    _stmt(t.body);
    _handlers.removeLast();
    if (cur != null) {
      _runFinallyOnce(fin);
      _jump(join, EdgeKind.normal);
    }

    // The catch clauses. A throw inside one still goes through the finally.
    if (hasCatch) {
      final dispatch = ctx.dispatch!;
      var catchAll = false;
      for (final c in t.catchClauses) {
        final entry = cfg.newBlock('catch ${c.exceptionType?.toSource() ?? 'any'}');
        _edge(dispatch, entry, EdgeKind.catchMatch, c.exceptionType?.toSource() ?? 'any');
        if (c.exceptionType == null) catchAll = true;
        cur = entry;
        if (fin != null) {
          // Only the finally part of the context applies inside a catch body.
          _handlers.add(_Try(hasCatch: false, fin: fin)..unwind = ctx.unwind);
        }
        _stmt(c.body);
        if (fin != null) _handlers.removeLast();
        if (cur != null) {
          _runFinallyOnce(fin);
          _jump(join, EdgeKind.normal);
        }
      }
      if (!catchAll) {
        // Not caught here: on through the finally to the outer handler.
        _edge(dispatch, ctx.unwind ?? _exceptionTarget(_handlers.length), EdgeKind.exception, 'no match');
      }
    }

    // The unwinding path: finally, then the exception goes on.
    final unwind = ctx.unwind;
    if (unwind != null && fin != null) {
      cur = unwind;
      _runFinallyOnce(fin);
      if (cur != null) _jump(_exceptionTarget(_handlers.length), EdgeKind.exception, 'rethrown after finally');
    }
    cur = join;
    _dropIfDead(join);
  }

  void _runFinallyOnce(ast.Block? fin) {
    if (fin == null || cur == null) return;
    _item('finally (copy)');
    _stmt(fin);
  }

  // --- switch ---------------------------------------------------------------

  void _switch(SwitchStatement s) {
    final labels = _takeLabels();
    _expr(s.expression);
    final after = cfg.newBlock('switch.after');
    _jumps.add(_Jump(labels, after, null, _handlers.length));
    var test = cur;
    var hasDefault = false;
    for (final m in s.members) {
      if (test == null) break;
      switch (m) {
        case SwitchPatternCase():
          final body = cfg.newBlock('case ${_short(m.guardedPattern.pattern.toSource())}');
          final next = cfg.newBlock('case.next');
          cur = test;
          _item('match ${_short(m.guardedPattern.toSource())}');
          final guard = m.guardedPattern.whenClause;
          final c = cur;
          if (c != null) {
            if (guard == null) {
              _edge(c, body, EdgeKind.caseMatch);
              _edge(c, next, EdgeKind.caseFail);
              cur = null;
            } else {
              // The pattern matched, then the guard decides.
              final g = cfg.newBlock('when');
              _edge(c, g, EdgeKind.caseMatch, 'pattern');
              _edge(c, next, EdgeKind.caseFail, 'pattern');
              cur = g;
              _cond(guard.expression, body, next);
            }
          }
          cur = body;
          for (final st in m.statements) {
            _stmt(st);
          }
          // DECISION: Dart 3 cases do not fall through: the end of a body
          // leaves the switch.
          _jump(after, EdgeKind.normal);
          test = next;
        case SwitchDefault():
          hasDefault = true;
          cur = test;
          _item('default');
          for (final st in m.statements) {
            _stmt(st);
          }
          _jump(after, EdgeKind.normal);
          test = null;
        default:
          _item('<unhandled ${m.runtimeType}>');
      }
    }
    _jumps.removeLast();
    // No case matched. DECISION: for a sealed type the analyzer proves the
    // switch exhaustive and this edge is infeasible; the CFG keeps it, as it
    // does not do that reasoning.
    if (test != null && !hasDefault) _edge(test, after, EdgeKind.caseFail, 'no case matched');
    cur = after;
    _dropIfDead(after);
  }

  String _short(String s) => s.length > 40 ? '${s.substring(0, 37)}...' : s;

  // --- expressions -----------------------------------------------------------

  /// Branches on a condition. && and || are control flow, not operators.
  void _cond(Expression e, Block thenB, Block elseB, {bool backEdgeTrue = false}) {
    switch (e) {
      case ParenthesizedExpression():
        _cond(e.expression, thenB, elseB);
      case BinaryExpression() when e.operator.type == TokenType.AMPERSAND_AMPERSAND:
        final rhs = cfg.newBlock('&&.rhs');
        _cond(e.leftOperand, rhs, elseB);
        cur = rhs;
        _cond(e.rightOperand, thenB, elseB);
      case BinaryExpression() when e.operator.type == TokenType.BAR_BAR:
        final rhs = cfg.newBlock('||.rhs');
        _cond(e.leftOperand, thenB, rhs);
        cur = rhs;
        _cond(e.rightOperand, thenB, elseB);
      case PrefixExpression() when e.operator.type == TokenType.BANG:
        _cond(e.operand, elseB, thenB);
      default:
        _expr(e);
        final c = cur;
        if (c == null) return;
        c.items.add('branch');
        _edge(c, thenB, backEdgeTrue ? EdgeKind.back : EdgeKind.ifTrue);
        _edge(c, elseB, EdgeKind.ifFalse);
        cur = null;
    }
  }

  /// Evaluates an expression in order of evaluation. Control flow inside
  /// expressions (&&, ||, ?:, ??, switch, await, throw) becomes blocks;
  /// everything else is recorded as an item of the current block.
  void _expr(Expression e) {
    if (cur == null) return;
    switch (e) {
      case ParenthesizedExpression():
        _expr(e.expression);
      case AwaitExpression():
        _expr(e.expression);
        _item('await ${_short(e.expression.toSource())}');
        _mayThrow();
        _suspend('await');
      case ThrowExpression():
        _expr(e.expression);
        _item('throw');
        final c = cur;
        if (c != null) _edge(c, _exceptionTarget(_handlers.length), EdgeKind.exception, 'throw');
        cur = null;
      case RethrowExpression():
        _item('rethrow');
        final c = cur;
        if (c != null) _edge(c, _exceptionTarget(_handlers.length), EdgeKind.exception, 'rethrow');
        cur = null;
      case BinaryExpression() when e.operator.type == TokenType.AMPERSAND_AMPERSAND || e.operator.type == TokenType.BAR_BAR:
        final t = cfg.newBlock('logic.true');
        final f = cfg.newBlock('logic.false');
        final join = cfg.newBlock('logic.join');
        _cond(e, t, f);
        t.items.add('true');
        f.items.add('false');
        _edge(t, join, EdgeKind.normal);
        _edge(f, join, EdgeKind.normal);
        cur = join;
      case BinaryExpression() when e.operator.type == TokenType.QUESTION_QUESTION:
        _expr(e.leftOperand);
        final rhs = cfg.newBlock('??.rhs');
        final join = cfg.newBlock('??.join');
        final c = cur;
        if (c != null) {
          _item('?? null?');
          _edge(c, rhs, EdgeKind.ifTrue, 'was null');
          _edge(c, join, EdgeKind.ifFalse, 'not null');
        }
        cur = rhs;
        _expr(e.rightOperand);
        _jump(join, EdgeKind.normal);
        cur = join;
      case ConditionalExpression():
        final thenB = cfg.newBlock('?.then');
        final elseB = cfg.newBlock('?.else');
        final join = cfg.newBlock('?.join');
        _cond(e.condition, thenB, elseB);
        cur = thenB;
        _expr(e.thenExpression);
        _jump(join, EdgeKind.normal);
        cur = elseB;
        _expr(e.elseExpression);
        _jump(join, EdgeKind.normal);
        cur = join;
      case SwitchExpression():
        _switchExpr(e);
      case MethodInvocation():
        final t = e.target;
        if (t != null) _expr(t);
        for (final a in e.argumentList.arguments) {
          _expr(a.argumentExpression);
        }
        _item(_short(e.toSource()));
        _mayThrow();
      case FunctionExpressionInvocation():
        _expr(e.function);
        for (final a in e.argumentList.arguments) {
          _expr(a.argumentExpression);
        }
        _item(_short(e.toSource()));
        _mayThrow();
      case InstanceCreationExpression():
        for (final a in e.argumentList.arguments) {
          _expr(a.argumentExpression);
        }
        _item(_short(e.toSource()));
        _mayThrow();
      case AssignmentExpression():
        _expr(e.rightHandSide);
        _item(_short(e.toSource()));
      case FunctionExpression():
        // DECISION: a closure is a value; its body is another graph.
        _item('closure');
      default:
        // Other expressions: evaluate the operands that are expressions.
        for (final c in e.childEntities) {
          if (c is Expression) _expr(c);
        }
    }
  }

  void _switchExpr(SwitchExpression e) {
    _expr(e.expression);
    final join = cfg.newBlock('switch.join');
    var test = cur;
    for (final c in e.cases) {
      if (test == null) break;
      final body = cfg.newBlock('=> ${_short(c.guardedPattern.pattern.toSource())}');
      final next = cfg.newBlock('case.next');
      cur = test;
      _item('match ${_short(c.guardedPattern.toSource())}');
      final guard = c.guardedPattern.whenClause;
      final blk = cur;
      if (blk != null) {
        if (guard == null) {
          _edge(blk, body, EdgeKind.caseMatch);
          _edge(blk, next, EdgeKind.caseFail);
        } else {
          final g = cfg.newBlock('when');
          _edge(blk, g, EdgeKind.caseMatch, 'pattern');
          _edge(blk, next, EdgeKind.caseFail, 'pattern');
          cur = g;
          _cond(guard.expression, body, next);
        }
      }
      cur = body;
      _expr(c.expression);
      _jump(join, EdgeKind.normal);
      test = next;
    }
    if (test != null) _edge(test, _exceptionTarget(_handlers.length), EdgeKind.exception, 'no case matched');
    cur = join;
    _dropIfDead(join);
  }

  /// The function is suspended and later resumed (await, yield).
  /// DECISION: suspension is an edge of its own kind to a fresh block. It
  /// adds no instruction (ADR-003): async and exceptions are edges.
  void _suspend(String what) {
    final c = cur;
    if (c == null) return;
    final next = cfg.newBlock('resume ($what)');
    _edge(c, next, EdgeKind.awaitResume, what);
    cur = next;
  }
}
