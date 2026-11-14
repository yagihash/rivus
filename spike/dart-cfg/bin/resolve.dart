// Can the analyzer follow calls across packages of a monorepo, and into
// generated code? Prints where each call of b/bin/main.dart is declared.
import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/dart/element/element.dart';

class _Calls extends RecursiveAstVisitor<void> {
  final calls = <MethodInvocation>[];
  @override
  void visitMethodInvocation(MethodInvocation node) {
    calls.add(node);
    super.visitMethodInvocation(node);
  }
}

String where(Element? e) {
  if (e == null) return '<unresolved>';
  final frag = e.firstFragment;
  final src = frag.libraryFragment?.source;
  return '${e.displayName} declared in ${src?.fullName}';
}

Future<void> main(List<String> args) async {
  final mono = Directory(args.isNotEmpty ? args.first : 'samples/mono').absolute.path;
  // One collection for the whole monorepo: the context of each package comes
  // from its package_config.json.
  final collection = AnalysisContextCollection(includedPaths: [mono]);
  stdout.writeln('contexts: ${collection.contexts.map((c) => c.contextRoot.root.path.split('/').last).toList()}');

  final main = '$mono/b/bin/main.dart';
  final ctx = collection.contextFor(main);
  final r = await ctx.currentSession.getResolvedUnit(main) as ResolvedUnitResult;
  final v = _Calls();
  r.unit.accept(v);
  for (final c in v.calls) {
    stdout.writeln('b: ${c.methodName.name} -> ${where(c.methodName.element)}');
  }

  // Follow into a: the body of parseUser calls the factory User.fromJson,
  // whose body calls a function of the generated part file.
  final aFile = '$mono/a/lib/a.dart';
  final a = await collection.contextFor(aFile).currentSession.getResolvedUnit(aFile) as ResolvedUnitResult;
  final av = _Calls();
  a.unit.accept(av);
  stdout.writeln('a: calls ${av.calls.map((c) => c.methodName.name).toList()}');

  final userFile = '$mono/a/lib/user.dart';
  final u = await collection.contextFor(userFile).currentSession.getResolvedUnit(userFile) as ResolvedUnitResult;
  final uv = _Calls();
  u.unit.accept(uv);
  for (final c in uv.calls) {
    stdout.writeln('user.dart: ${c.methodName.name} -> ${where(c.methodName.element)}');
  }
  stdout.writeln('errors: ${[...r.diagnostics, ...a.diagnostics, ...u.diagnostics].where((d) => d.diagnosticCode.type.name == 'COMPILE_TIME_ERROR').map((d) => d.message).toList()}');
}
