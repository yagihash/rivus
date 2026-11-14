import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';

import 'cfg.dart';

class _Functions extends RecursiveAstVisitor<void> {
  final found = <(String, FunctionBody)>[];
  var _classes = <String>[];

  @override
  void visitClassDeclaration(ClassDeclaration node) {
    _classes.add(node.declaredFragment?.name ?? '?');
    super.visitClassDeclaration(node);
    _classes.removeLast();
  }

  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    final name = node.name.lexeme;
    found.add((_classes.isEmpty ? name : '${_classes.last}.$name', node.functionExpression.body));
    // Not descending: local functions and closures are graphs of their own.
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    final name = node.name.lexeme;
    found.add((_classes.isEmpty ? name : '${_classes.last}.$name', node.body));
  }
}

/// Resolves a file the way the extractor would, and returns the compile time
/// errors with it: a spike that analyses code that does not compile proves
/// nothing.
Future<(ResolvedUnitResult, List<String>)> resolve(String path) async {
  final file = File(path).absolute.path;
  final collection = AnalysisContextCollection(includedPaths: [file]);
  final context = collection.contextFor(file);
  final result = await context.currentSession.getResolvedUnit(file);
  if (result is! ResolvedUnitResult) {
    throw StateError('could not resolve $file: $result');
  }
  final errors = [
    for (final d in result.diagnostics)
      if (d.diagnosticCode.type.name == 'COMPILE_TIME_ERROR') d.message,
  ];
  return (result, errors);
}

/// The graph of every function of a file, by name.
Future<Map<String, Cfg>> cfgsOf(String path) async {
  final (result, errors) = await resolve(path);
  if (errors.isNotEmpty) {
    throw StateError('compile time errors in $path: $errors');
  }
  final fns = _Functions();
  result.unit.accept(fns);
  return {for (final (name, body) in fns.found) name: CfgBuilder(name).build(body)};
}
