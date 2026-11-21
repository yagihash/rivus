// A trial of docs/DESIGN-observed-calls.md (method B; not run by CI): puts a call of `__rv.rvEnter()` at the start
// of every function body of the Dart files that it is given (a copy of the cells, never the originals), so that every
// call that happens leaves the stack of its callee: the callee and the place of the call. Syntax only (no types).
//
// usage: dart --packages=<extractor/dart/.dart_tool/package_config.json> instrument.dart <dir of lib files>...
import 'dart:io';

import 'package:analyzer/dart/analysis/features.dart';
import 'package:analyzer/dart/analysis/utilities.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/source/line_info.dart';

class _V extends RecursiveAstVisitor<void> {
  _V(this.lines);
  final LineInfo lines;
  final edits = <(int, int, String)>[]; // offset, end, text

  /// The call that records the entry: it carries where the function starts in the ORIGINAL file (the line and the
  /// column that the extractor gives the function: `funcs.line` and `funcs.col` of the graph), so that the callee is
  /// not guessed from the name in the stack (OBS-1).
  String _enter(FunctionBody body) {
    // What the extractor gives as the start (funcs.line, funcs.col): a top level function, a method and a
    // constructor start where the declaration starts (with its annotations); a local function and a closure start
    // where the parameters start (the function expression).
    final p = body.parent;
    int offset;
    if (p is FunctionExpression && p.parent is FunctionDeclaration && p.parent!.parent is CompilationUnit) {
      offset = p.parent!.offset;
    } else if (p is MethodDeclaration || p is ConstructorDeclaration) {
      offset = p!.offset;
    } else {
      offset = p?.offset ?? body.offset;
    }
    final loc = lines.getLocation(offset);
    return '__rv.rvEnter(${loc.lineNumber}, ${loc.columnNumber});';
  }

  @override
  void visitBlockFunctionBody(BlockFunctionBody node) {
    edits.add((node.block.leftBracket.end, node.block.leftBracket.end, ' ${_enter(node)}'));
    super.visitBlockFunctionBody(node);
  }

  @override
  void visitExpressionFunctionBody(ExpressionFunctionBody node) {
    final isVoid = _isVoid(node);
    // `=> e;` becomes `{ __rv.rvEnter(); e; }` or `{ __rv.rvEnter(); return e; }`
    edits.add((node.functionDefinition.offset, node.expression.offset, '{ ${_enter(node)} ${isVoid ? '' : 'return '}'));
    edits.add((node.expression.end, node.end, '; }'));
    super.visitExpressionFunctionBody(node);
  }
}

bool _isVoid(ExpressionFunctionBody body) {
  final p = body.parent;
  if (p is FunctionExpression) {
    final pp = p.parent;
    if (pp is FunctionDeclaration) return pp.returnType?.toSource() == 'void';
    return false;
  }
  if (p is MethodDeclaration) return p.returnType?.toSource() == 'void';
  return false;
}

void main(List<String> args) {
  for (final dir in args) {
    for (final e in Directory(dir).listSync(recursive: true)) {
      if (e is! File || !e.path.endsWith('.dart')) continue;
      final src = e.readAsStringSync();
      final r = parseString(content: src, featureSet: FeatureSet.latestLanguageVersion(), throwIfDiagnostics: false);
      if (r.unit.directives.any((d) => d is PartOfDirective)) continue;
      final v = _V(r.lineInfo);
      r.unit.accept(v);
      final edits = v.edits..sort((a, b) => b.$1.compareTo(a.$1));
      var out = src;
      for (final (a, b, t) in edits) {
        out = out.replaceRange(a, b, t);
      }
      final imports = r.unit.directives.whereType<ImportDirective>().toList();
      // all the edits are in bodies, after the directives: the offset of the first import is still right
      final at = imports.isEmpty ? 0 : imports.first.offset;
      out = out.replaceRange(at, at, "import 'package:zoo_mark/zoo_mark.dart' as __rv; ");
      e.writeAsStringSync(out);
    }
  }
}
