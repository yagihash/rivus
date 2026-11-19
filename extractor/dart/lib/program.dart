// The program being extracted: the units, the texts the core will ask for,
// and the things that need the whole program (the classes, for the call
// graph).
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/token.dart';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/source/line_info.dart';

import 'irjson.dart';
import 'names.dart';

class MethodImpl {
  MethodImpl(this.cls, this.id);
  final InterfaceElement cls;
  final String id;
}

/// The texts, grouped as the stream has them: the same text for every
/// position it belongs to.
class Texts {
  /// Files of dependencies: the core wants no text of them.
  final Set<String> skip = {};
  final Map<String, Map<String, Map<String, Set<int>>>> byKind = {}; // kind -> file -> text -> offsets
  final Map<String, (String call, List<String> args)> calls = {}; // "file\u0000offset"
  final List<(String id, String body, String source)> funcs = [];

  void add(String kind, String file, String text, int offset) {
    if (text.isEmpty || skip.contains(file)) return;
    ((byKind[kind] ??= {})[file] ??= {})[text] ??= <int>{};
    byKind[kind]![file]![text]!.add(offset);
  }

  void write(StreamOut out) {
    for (final kind in ['cond', 'expr', 'snippet']) {
      final files = byKind[kind];
      if (files == null) continue;
      for (final file in files.keys.toList()..sort()) {
        final texts = files[file]!;
        for (final text in texts.keys.toList()..sort()) {
          out.text(kind, file, text, texts[text]!.toList()..sort());
        }
      }
    }
    final keys = calls.keys.toList()..sort();
    for (final k in keys) {
      final (call, args) = calls[k]!;
      final i = k.indexOf('\u0000');
      out.callText(k.substring(0, i), int.parse(k.substring(i + 1)), call, args);
    }
    for (final (id, body, source) in funcs) {
      out.funcText(id, body, source);
    }
  }
}

class Program {
  Program(this.root, this.out);
  final String root;
  final StreamOut out;
  final texts = Texts();
  final methods = <String, List<MethodImpl>>{}; // by member name
  final types = <String, Set<String>>{}; // a class -> the types it extends, implements, mixes in
  final bodies = <String>{}; // the symbols of what has a body here
  /// The constructors that are written out: the ones that have something to do, and the ones that a
  /// constructor which has something to do calls (the implicit `super()` runs the field initializers
  /// of the base class, even when the base constructor itself is empty).
  final ctorsOut = <String>{};
  final ctorCalls = <String, Set<String>>{}; // a constructor -> the constructors it calls

  /// Closes [ctorsOut] over [ctorCalls]: a constructor that calls one that is written out is written out.
  void closeCtors() {
    var changed = true;
    while (changed) {
      changed = false;
      for (final e in ctorCalls.entries) {
        if (!ctorsOut.contains(e.key) && e.value.any(ctorsOut.contains)) {
          ctorsOut.add(e.key);
          changed = true;
        }
      }
    }
  }
  final globals = <String, (String type, Pos pos)>{};
  var maxSourceLines = 80;

  void noteGlobal(String name, String type, Pos pos) => globals.putIfAbsent(name, () => (type, pos));

  /// The functions a call to the method may reach, among the code here: the
  /// method itself and what overrides it (class hierarchy analysis).
  List<String> candidates(ExecutableElement m, InterfaceElement? receiver) {
    final base = m.baseElement;
    final out = <String>{};
    final self = funcId(base);
    if (bodies.contains(self)) out.add(self);
    final name = (base.name ?? '').replaceAll('=', '');
    for (final impl in methods[name] ?? const <MethodImpl>[]) {
      final start = receiver ?? base.enclosingElement;
      if (start is! InterfaceElement) continue;
      if (impl.cls == start || impl.cls.allSupertypes.any((s) => s.element == start)) out.add(impl.id);
    }
    return out.toList()..sort();
  }
}

/// A unit being lowered: its syntax, and the conversion of positions.
class UnitInfo {
  UnitInfo(this.prog, this.file, this.unit, this.source, this.lines, {this.dep = false}) {
    if (dep) prog.texts.skip.add(file);
  }
  final bool dep; // a file of a dependency: read so that calls into it are followed
  final Program prog;
  final String file; // relative to the root
  final CompilationUnit unit;
  final String source;
  final LineInfo lines;

  Pos at(int offset) {
    final loc = lines.getLocation(offset);
    return Pos(file, offset, loc.lineNumber, loc.columnNumber);
  }

  /// The position of a node, and the texts the core will want for it.
  Pos pos(AstNode n, int offset) {
    final p = at(offset);
    final s = snippetOf(n);
    prog.texts.add('snippet', file, s, offset);
    prog.texts.add('cond', file, condOf(n), offset);
    return p;
  }

  /// A call: its position is the opening parenthesis, and the core also gets
  /// the normalized expression and the text of the call and of its arguments.
  Pos callPos(AstNode call, ArgumentList args, List<AstNode?> slots) {
    final off = args.leftParenthesis.offset;
    final p = pos(call, off);
    prog.texts.add('expr', file, normalized(call.beginToken, call.endToken), off);
    if (!prog.texts.skip.contains(file)) prog.texts.calls['$file\u0000$off'] = (
      raw(call),
      [for (final s in slots) s == null ? '' : raw(s)],
    );
    return p;
  }

  /// The position of a place where a field is read or written.
  Pos fieldPos(AstNode n, int offset) {
    final p = pos(n, offset);
    prog.texts.add('expr', file, normalized(statementOf(n).beginToken, statementOf(n).endToken), offset);
    return p;
  }

  String raw(AstNode n) => source.substring(n.offset, n.end);

  AstNode statementOf(AstNode n) {
    for (AstNode? x = n; x != null; x = x.parent) {
      if (x is Statement && x is! Block) return x;
      if (x is FieldDeclaration || x is TopLevelVariableDeclaration || x is Declaration && x is! VariableDeclaration && x is! FormalParameter) {
        return x;
      }
    }
    return n;
  }

  /// The statement around the node, as a reader wants it: the header of a
  /// compound statement, the whole of a simple one.
  String snippetOf(AstNode n) {
    for (AstNode? x = n; x != null; x = x.parent) {
      int? end;
      switch (x) {
        case IfStatement():
          end = x.rightParenthesis.end;
        case WhileStatement():
          end = x.rightParenthesis.end;
        case ForStatement():
          end = x.rightParenthesis.end;
        case SwitchStatement():
          end = x.rightParenthesis.end;
        case TryStatement():
          end = x.body.offset;
        case DoStatement():
          end = x.body.offset;
        case Block():
        case SwitchMember():
        case CatchClause():
          continue;
        case Statement():
          end = x.end;
        case FunctionDeclaration():
          end = x.functionExpression.body.offset;
        case MethodDeclaration():
          end = x.body.offset;
        case ConstructorDeclaration():
          end = x.body.offset;
        case FieldDeclaration():
          end = x.end;
        case TopLevelVariableDeclaration():
          end = x.end;
        default:
          continue;
      }
      return _dedent(source.substring(x.offset, end).trimRight());
    }
    return '';
  }

  String _dedent(String s) => s;

  /// The condition of the if, the loop or the conditional expression the node
  /// is in.
  String condOf(AstNode n) {
    for (AstNode? x = n; x != null; x = x.parent) {
      switch (x) {
        case IfStatement():
          return raw(x.expression);
        case WhileStatement():
          return raw(x.condition);
        case DoStatement():
          return raw(x.condition);
        case ForStatement():
          final p = x.forLoopParts;
          if (p is ForParts && p.condition != null) return raw(p.condition!);
        case ConditionalExpression():
          return raw(x.condition);
        default:
      }
    }
    return '';
  }

  /// The tokens from [from] to [to], without comments, joined by spaces.
  String normalized(Token from, Token to) {
    final b = StringBuffer();
    for (Token? t = from; t != null; t = t.next) {
      if (b.isNotEmpty) b.write(' ');
      b.write(t.lexeme);
      if (t == to || t.type == TokenType.EOF) break;
    }
    return b.toString();
  }

  String bodyText(AstNode n) => normalized(n.beginToken, n.endToken);

  String sourceText(AstNode n) {
    final lines = raw(n).split('\n');
    if (lines.length > prog.maxSourceLines) {
      return [...lines.take(prog.maxSourceLines), '// ... (cut)'].join('\n');
    }
    return lines.join('\n');
  }

  void funcTexts(String id, AstNode n) => prog.texts.skip.contains(file) ? null : prog.texts.funcs.add((id, bodyText(n), sourceText(n)));
}
