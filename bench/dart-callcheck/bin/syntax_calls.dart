// The calls that the Dart source writes, one JSON object per line: {file, offset, start, line, kind, owner, owner_line}.
//
//   dart run bin/syntax_calls.dart <directory>      (bench/dart-callcheck/callcheck.py runs it)
//
// This file does not use the code of the extractor (extractor/dart): "what is a call in the syntax" is defined here, from
// the language, so that a call the extractor leaves out is not left out here too.
//
// What is counted, a "call written in the syntax": an argument list that the programmer wrote, and that runs code.
//   MethodInvocation                       f(x), o.m(x), o?.m(x), C(x), C.named(x), p.f(x), the sections of a cascade (..m(x))
//   FunctionExpressionInvocation           (f)(x), f()(x), a call of a value
//   InstanceCreationExpression             new C(x), const C(x), and C(x) once the analyzer knows it is a constructor
//   DotShorthandInvocation                 .make(x)
//   DotShorthandConstructorInvocation      .new(x), .named(x)
//   SuperConstructorInvocation             `: super(x)` of the initializer list
//   RedirectingConstructorInvocation       `: this(x)` of the initializer list
//   EnumConstantArguments                  `A(1)` of an enum constant (a call of the generative constructor)
// What is not counted: the code that runs without an argument list in the source (a getter, a setter, an operator, an index,
// `toString` of an interpolation, an implicit `call`, an implicit super constructor call), an annotation (`@A(x)`, a
// constant for the tools), an extension override (`E(x).m()`: the `E(x)` is not a call), an object pattern `C(x: 1)`.
// A cascade is not counted as a node: each section is its own call (a cascade of 3 sections has 3 positions).
//
// The position: the offset of the "(" that opens the argument list (for `.new(x)`, `A(1)` and `super(x)` too), 0-based, in
// UTF-16 units of the file (what the analyzer, and the extractor's Pos.Offset, count). `owner`: the declaration that holds
// the call (the outermost member: a function, a method, a constructor, a field, a top-level variable, an enum constant),
// so that "the functions that have a mismatch" can be counted; a closure belongs to the member around it.
//
// The files: every .dart file under the directory, except the directories that the package's build does not ship
// (test, integration_test, benchmark, tool and example of a package, the directory that holds a pubspec.yaml), the
// hidden directories, and build/. This is the same rule as the one of the extractor README, written again here.
import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/source/line_info.dart';
import 'package:path/path.dart' as p;

const _skipped = {'test', 'integration_test', 'benchmark', 'tool', 'example'};

Future<void> main(List<String> args) async {
  final root = p.normalize(p.absolute(args.isEmpty ? '.' : args.first));
  final collection = AnalysisContextCollection(includedPaths: [root]);
  final done = <String>{};
  for (final ctx in collection.contexts) {
    for (final path in ctx.contextRoot.analyzedFiles()) {
      if (!path.endsWith('.dart') || !p.isWithin(root, path) || !done.add(path)) continue;
      if (_isSkipped(root, path)) continue;
      final r = await ctx.currentSession.getResolvedUnit(path);
      if (r is! ResolvedUnitResult) continue;
      final rel = p.relative(path, from: root);
      final v = _Calls(rel, r.lineInfo);
      r.unit.accept(v);
      for (final c in v.calls) {
        stdout.writeln(jsonEncode(c));
      }
    }
  }
}

bool _isSkipped(String root, String path) {
  final parts = p.split(p.relative(path, from: root));
  for (var i = 0; i < parts.length - 1; i++) {
    final d = parts[i];
    if (d.startsWith('.') || d == 'build') return true;
    // the first directory under a package
    if (_skipped.contains(d) && File(p.joinAll([root, ...parts.take(i), 'pubspec.yaml'])).existsSync()) return true;
  }
  return false;
}

class _Calls extends RecursiveAstVisitor<void> {
  _Calls(this.file, this.lines);
  final String file;
  final LineInfo lines;
  final calls = <Map<String, Object?>>[];

  void _add(AstNode node, ArgumentList args, String kind) {
    final off = args.leftParenthesis.offset;
    final owner = _owner(node);
    calls.add({
      'file': file,
      'offset': off,
      'start': node.offset,
      'line': lines.getLocation(off).lineNumber,
      'kind': kind,
      'owner': owner == null ? '' : _label(owner),
      'owner_line': owner == null ? 0 : lines.getLocation(owner.offset).lineNumber,
    });
  }

  AstNode? _owner(AstNode n) {
    AstNode? best;
    for (AstNode? x = n; x != null; x = x.parent) {
      if (x is ClassMember || x is EnumConstantDeclaration || x is TopLevelVariableDeclaration) best = x;
      if (x is FunctionDeclaration && x.parent is CompilationUnit) best = x;
    }
    return best;
  }

  String _label(AstNode n) {
    final s = n.toSource().replaceAll(RegExp(r'\s+'), ' ');
    return s.length > 60 ? s.substring(0, 60) : s;
  }

  @override
  void visitAnnotation(Annotation node) {} // not a call of the program

  @override
  void visitMethodInvocation(MethodInvocation node) {
    _add(node, node.argumentList, 'method');
    super.visitMethodInvocation(node);
  }

  @override
  void visitFunctionExpressionInvocation(FunctionExpressionInvocation node) {
    _add(node, node.argumentList, 'function_expression');
    super.visitFunctionExpressionInvocation(node);
  }

  @override
  void visitInstanceCreationExpression(InstanceCreationExpression node) {
    _add(node, node.argumentList, 'creation');
    super.visitInstanceCreationExpression(node);
  }

  @override
  void visitDotShorthandInvocation(DotShorthandInvocation node) {
    _add(node, node.argumentList, 'dot_shorthand');
    super.visitDotShorthandInvocation(node);
  }

  @override
  void visitDotShorthandConstructorInvocation(DotShorthandConstructorInvocation node) {
    _add(node, node.argumentList, 'dot_shorthand_constructor');
    super.visitDotShorthandConstructorInvocation(node);
  }

  @override
  void visitSuperConstructorInvocation(SuperConstructorInvocation node) {
    _add(node, node.argumentList, 'super_initializer');
    super.visitSuperConstructorInvocation(node);
  }

  @override
  void visitRedirectingConstructorInvocation(RedirectingConstructorInvocation node) {
    _add(node, node.argumentList, 'redirecting_initializer');
    super.visitRedirectingConstructorInvocation(node);
  }

  @override
  void visitEnumConstantArguments(EnumConstantArguments node) {
    _add(node, node.argumentList, 'enum_constant');
    super.visitEnumConstantArguments(node);
  }
}
