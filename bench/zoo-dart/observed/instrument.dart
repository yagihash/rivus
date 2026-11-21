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

class _V extends RecursiveAstVisitor<void> {
  final edits = <(int, int, String)>[]; // offset, end, text
  @override
  void visitBlockFunctionBody(BlockFunctionBody node) {
    edits.add((node.block.leftBracket.end, node.block.leftBracket.end, ' __rv.rvEnter();'));
    super.visitBlockFunctionBody(node);
  }

  @override
  void visitExpressionFunctionBody(ExpressionFunctionBody node) {
    final isVoid = _isVoid(node);
    // `=> e;` becomes `{ __rv.rvEnter(); e; }` or `{ __rv.rvEnter(); return e; }`
    edits.add((node.functionDefinition.offset, node.expression.offset, '{ __rv.rvEnter(); ${isVoid ? '' : 'return '}'));
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
      final v = _V();
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
