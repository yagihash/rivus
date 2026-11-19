// The driver: resolves the code, lowers every function with a body and writes
// the stream.
import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context.dart';
import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';
import 'package:path/path.dart' as p;

import 'irjson.dart';
import 'lower.dart';
import 'names.dart';
import 'program.dart';

/// Extracts the Dart code under [root] (the files that are not under a hidden
/// or a build directory) and writes the stream to [sink].
Future<void> extract(String root, StringSink sink, {bool deps = true, List<String> allowDepsUnder = const []}) async {
  root = p.normalize(p.absolute(root));
  final out = StreamOut(sink);
  final prog = Program(root, out);
  final collection = AnalysisContextCollection(includedPaths: [root]);

  // Resolve everything first: the call graph needs all of the classes.
  final units = <UnitInfo>[];
  // The files of the project: the ones the analysis takes, and the ones that
  // analysis_options.yaml leaves out of it (the generated code: *.g.dart and
  // *.freezed.dart are the usual `exclude`), which are code of the project all the same.
  final paths = <String>{};
  for (final context in collection.contexts) {
    for (final path in context.contextRoot.analyzedFiles()) {
      if (path.endsWith('.dart') && p.isWithin(root, path) && !_skipped(root, path)) paths.add(path);
    }
  }
  for (final e in Directory(root).listSync(recursive: true, followLinks: false)) {
    if (e is File && e.path.endsWith('.dart') && !_skipped(root, e.path)) paths.add(p.normalize(e.path));
  }
  for (final path in paths.toList()..sort()) {
    final context = _contextOf(collection, path);
    if (context == null) continue; // not in any package of the project
    final r = await context.currentSession.getResolvedUnit(path);
    if (r is! ResolvedUnitResult) {
      out.loadError(p.relative(path, from: root), 'could not be resolved');
      continue;
    }
    for (final d in r.diagnostics) {
      if (d.diagnosticCode.type.name == 'COMPILE_TIME_ERROR') {
        out.loadError(p.relative(path, from: root), d.message);
      }
    }
    units.add(UnitInfo(prog, p.relative(path, from: root), r.unit, r.content, r.lineInfo));
  }
  // The dependencies (not the SDK): the libraries the code imports, and what
  // they import. Their functions are marked dep; the core keeps the ones the
  // code reaches.
  final refused = <String, String>{}; // the directory of a dependency that was not read -> a file in it
  if (deps) {
    final seen = <String>{};
    final files = <String>[];
    void visit(LibraryElement lib) {
      if (lib.uri.scheme == 'dart' || !seen.add(lib.uri.toString())) return;
      for (final f in lib.fragments) {
        final path = f.source.fullName;
        if (!p.isWithin(root, path)) files.add(path);
      }
      for (final i in lib.firstFragment.libraryImports) {
        final l = i.importedLibrary;
        if (l != null) visit(l);
      }
      for (final e in lib.firstFragment.libraryExports) {
        final l = e.exportedLibrary;
        if (l != null) visit(l);
      }
    }
    for (final context in collection.contexts) {
      for (final u in List.of(units)) {
        final r = await context.currentSession.getResolvedUnit(p.join(root, u.file));
        if (r is ResolvedUnitResult) visit(r.libraryElement);
      }
    }
    final done = <String>{};
    final trusted = _trustedDependencyRoots(allowDepsUnder);
    for (final path in files) {
      if (!path.endsWith('.dart') || !done.add(path)) continue;
      // Where a dependency is, is said by the package_config.json of the project, which is not trusted: it can
      // point anywhere. What is read is under the project, the pub cache, the SDKs, or what is asked for.
      if (!trusted.any((r) => p.equals(r, path) || p.isWithin(r, path))) {
        refused.putIfAbsent(_packageDir(path), () => path);
        continue;
      }
      final context = collection.contexts.first;
      final r = await context.currentSession.getResolvedUnit(path);
      if (r is! ResolvedUnitResult) continue;
      units.add(UnitInfo(prog, path, r.unit, r.content, r.lineInfo, dep: true));
    }
  }
  for (final e in refused.entries) {
    out.loadError(e.key,
        'a dependency outside the project, the pub cache and the SDKs is not read (what package_config.json points to is not trusted): --allow-deps-under ${p.dirname(e.key)} reads it');
  }
  units.sort((a, b) => a.file.compareTo(b.file));
  for (final u in units) {
    u.unit.accept(_Index(prog));
  }
  prog.closeCtors();
  for (final u in units) {
    u.unit.accept(_Lower(prog, u));
  }
  for (final e in prog.globals.entries) {
    out.global(e.key, e.value.$1, e.value.$2);
  }
  for (final id in prog.types.keys.toList()..sort()) {
    out.type(id, prog.types[id]!.toList()..sort());
  }
  prog.texts.write(out);
}

/// The directories that a dependency may be read from, besides the project itself: the pub cache, the
/// SDK that runs the extractor (and the Flutter that holds it, whose packages are `package:flutter`),
/// and what the user asked for.
List<String> _trustedDependencyRoots(List<String> extra) {
  final env = Platform.environment;
  final roots = <String>[];
  final home = env['HOME'] ?? env['USERPROFILE'];
  final cache = env['PUB_CACHE'] ??
      (Platform.isWindows ? (env['LOCALAPPDATA'] == null ? null : p.join(env['LOCALAPPDATA']!, 'Pub', 'Cache')) : (home == null ? null : p.join(home, '.pub-cache')));
  if (cache != null) roots.add(p.normalize(p.absolute(cache)));
  final sdk = p.dirname(p.dirname(Platform.resolvedExecutable));
  roots.add(sdk);
  // <flutter>/bin/cache/dart-sdk
  if (p.basename(sdk) == 'dart-sdk' && p.basename(p.dirname(sdk)) == 'cache') roots.add(p.dirname(p.dirname(p.dirname(sdk))));
  final flutter = env['FLUTTER_ROOT'];
  if (flutter != null && flutter.isNotEmpty) roots.add(p.normalize(p.absolute(flutter)));
  roots.addAll(extra.map((d) => p.normalize(p.absolute(d))));
  return roots;
}

/// The directory of the package a file is in: the one above its `lib` (or else the file's own).
String _packageDir(String path) {
  final parts = p.split(path);
  final i = parts.lastIndexOf('lib');
  return i > 0 ? p.joinAll(parts.sublist(0, i)) : p.dirname(path);
}

/// The context of the package the file is in: the one whose root is the closest
/// above it. (`contextFor` does not find a file that analysis_options.yaml leaves out.)
AnalysisContext? _contextOf(AnalysisContextCollection collection, String path) {
  AnalysisContext? best;
  for (final c in collection.contexts) {
    final r = c.contextRoot.root.path;
    if (p.isWithin(r, path) && (best == null || r.length > best.contextRoot.root.path.length)) best = c;
  }
  return best;
}

bool _skipped(String root, String path) {
  final rel = p.relative(path, from: root);
  final parts = p.split(rel);
  // hidden directories (.dart_tool, .git), and `build` at the top (what the tools write): a directory
  // called build deeper in the package is code (a `lib/src/build/command.dart` of a build tool)
  return parts.any((s) => s.startsWith('.')) || (parts.length > 1 && parts.first == 'build');
}

/// Pass 1: which functions have a body, and which class has which method.
class _Index extends RecursiveAstVisitor<void> {
  _Index(this.prog);
  final Program prog;

  void _note(Element? el, FunctionBody? body) {
    if (el == null || body == null || body is EmptyFunctionBody || body is NativeFunctionBody) return;
    final base = el.baseElement;
    final id = funcId(base);
    prog.bodies.add(id);
    final cls = base.enclosingElement;
    if (cls is InterfaceElement && base is! ConstructorElement) {
      (prog.methods[(base.name ?? '').replaceAll('=', '')] ??= []).add(MethodImpl(cls, id));
    }
  }

  /// What the class is a subtype of, directly: the rules ask for the
  /// subclasses of a framework's class (the services of grpc).
  void _type(InterfaceElement? el) {
    if (el == null) return;
    final supers = <DartType?>[el.supertype, ...el.interfaces, ...el.mixins];
    if (el is MixinElement) supers.addAll(el.superclassConstraints);
    final out = prog.types[typeId(el.thisType)] ??= {};
    for (final s in supers) {
      final id = typeId(s);
      if (id.isNotEmpty && id != 'dart:core#Object' && id != 'dynamic') out.add(id);
    }
  }

  @override
  void visitClassDeclaration(ClassDeclaration node) {
    final cls = node.declaredFragment?.element;
    _type(cls);
    // a class that declares no constructor has the one that Dart makes
    final members = node.body is BlockClassBody ? (node.body as BlockClassBody).members : const <ClassMember>[];
    if (cls != null && !members.any((m) => m is ConstructorDeclaration)) {
      final ctor = cls.unnamedConstructor;
      if (ctor != null) _noteCtor(ctor, own: _hasFieldInit(cls), calls: ctor.superConstructor);
    }
    super.visitClassDeclaration(node);
  }

  bool _hasFieldInit(InterfaceElement cls) =>
      cls.fields.any((f) => !f.isStatic && f.hasInitializer) ||
      (cls is ClassElement && cls.mixins.any((m) => m.element.fields.any((f) => !f.isStatic && f.hasInitializer)));

  void _noteCtor(ConstructorElement ctor, {required bool own, ConstructorElement? calls}) {
    final id = funcId(ctor);
    if (own) prog.ctorsOut.add(id);
    if (calls != null) (prog.ctorCalls[id] ??= {}).add(funcId(calls));
  }

  @override
  void visitMixinDeclaration(MixinDeclaration node) {
    _type(node.declaredFragment?.element);
    super.visitMixinDeclaration(node);
  }

  @override
  void visitEnumDeclaration(EnumDeclaration node) {
    _type(node.declaredFragment?.element);
    super.visitEnumDeclaration(node);
  }

  @override
  void visitExtensionTypeDeclaration(ExtensionTypeDeclaration node) {
    _type(node.declaredFragment?.element);
    super.visitExtensionTypeDeclaration(node);
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    _note(node.declaredFragment?.element, node.body);
  }

  @override
  void visitConstructorDeclaration(ConstructorDeclaration node) {
    _note(node.declaredFragment?.element, node.body);
    final el = node.declaredFragment?.element;
    if (el == null || node.factoryKeyword != null) return;
    // a constructor with nothing in it still sets the fields that have an initializer
    final own = node.body is! EmptyFunctionBody && node.body is! NativeFunctionBody ||
        node.initializers.isNotEmpty ||
        node.parameters.parameters.isNotEmpty ||
        _hasFieldInit(el.enclosingElement);
    // the constructor that makes the object: the one that super(...) or this.named(...) names, or the
    // one of the superclass that is called without being written
    ConstructorElement? calls;
    for (final i in node.initializers) {
      if (i is SuperConstructorInvocation) calls = i.element;
      if (i is RedirectingConstructorInvocation) calls = i.element;
    }
    _noteCtor(el, own: own, calls: calls ?? (node.initializers.any((i) => i is SuperConstructorInvocation || i is RedirectingConstructorInvocation) ? null : el.superConstructor));
  }

  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    if (node.parent is CompilationUnit) _note(node.declaredFragment?.element, node.functionExpression.body);
  }
}

/// What a function holds in a cell: the variables that a closure captures
/// and that are assigned after their declaration.
class _Scan extends RecursiveAstVisitor<void> {
  final captured = <Object>{};
  final assigned = <Object>{};
  final _declIn = <Object, AstNode>{};
  final _stack = <AstNode>[];

  void _push(AstNode n, void Function() f) {
    _stack.add(n);
    f();
    _stack.removeLast();
  }

  @override
  void visitFunctionExpression(FunctionExpression node) => _push(node, () => super.visitFunctionExpression(node));

  @override
  void visitVariableDeclaration(VariableDeclaration node) {
    final el = node.declaredFragment?.element;
    if (el != null && _stack.isNotEmpty) _declIn[el.baseElement] = _stack.last;
    super.visitVariableDeclaration(node);
  }

  @override
  void visitFormalParameterList(FormalParameterList node) {
    for (final fp in node.parameters) {
      _declare(fp.declaredFragment?.element);
    }
    super.visitFormalParameterList(node);
  }

  void _declare(Element? el) {
    if (el != null && _stack.isNotEmpty) _declIn[el.baseElement] = _stack.last;
  }

  @override
  void visitSimpleIdentifier(SimpleIdentifier node) {
    final el = node.element?.baseElement;
    if (el != null && _stack.isNotEmpty && _declIn[el] != null && _declIn[el] != _stack.last) captured.add(el);
    super.visitSimpleIdentifier(node);
  }

  void _write(Expression? target) {
    var t = target;
    while (t is ParenthesizedExpression) {
      t = t.expression;
    }
    if (t is SimpleIdentifier) {
      final el = t.element?.baseElement;
      if (el != null) assigned.add(el);
    }
  }

  @override
  void visitAssignmentExpression(AssignmentExpression node) {
    _write(node.leftHandSide);
    super.visitAssignmentExpression(node);
  }

  /// `(a, b) = value;`: the variables of the pattern are written.
  @override
  void visitAssignedVariablePattern(AssignedVariablePattern node) {
    final el = node.element?.baseElement;
    if (el != null) assigned.add(el);
    super.visitAssignedVariablePattern(node);
  }

  /// `for (x in xs)`: x is written at every step.
  @override
  void visitForEachPartsWithIdentifier(ForEachPartsWithIdentifier node) {
    _write(node.identifier);
    super.visitForEachPartsWithIdentifier(node);
  }

  @override
  void visitPrefixExpression(PrefixExpression node) {
    final t = node.operator.lexeme;
    if (t == '++' || t == '--') _write(node.operand);
    super.visitPrefixExpression(node);
  }

  @override
  void visitPostfixExpression(PostfixExpression node) {
    final t = node.operator.lexeme;
    if (t == '++' || t == '--') _write(node.operand);
    super.visitPostfixExpression(node);
  }
}

/// Pass 2: lowers every top level function, method and constructor with a
/// body. Local functions and closures are lowered with the function around.
class _Lower extends RecursiveAstVisitor<void> {
  _Lower(this.prog, this.unit);
  final Program prog;
  final UnitInfo unit;

  /// The initializer of a variable of the library (or a static field) is a function of its
  /// own, which stores the value in the variable: the program starts there too (the
  /// variable is set when it is first read, and the code that reads it does not ask when).
  void _initializer(VariableDeclaration v) {
    final init = v.initializer;
    final el = v.declaredFragment?.element;
    if (init == null || el == null) return;
    final id = '${symbolOf(el)}\$init';
    final out = FuncOut(id)
      ..symbol = ''
      ..dep = unit.dep
      ..entry = true
      ..init = true;
    out.pos = unit.pos(v, v.offset);
    final scan = _Scan();
    scan._stack.add(v);
    v.accept(scan);
    final f = FnLower(prog, unit, out, cells: scan.captured, assigned: scan.assigned);
    f.lowerInit(
      at: unit.at(v.offset),
      body: () {
        final value = f.expr(init);
        globalStore(f, el, value, f.pos(v));
      },
    );
    unit.funcTexts(id, v);
    prog.out.func(out);
  }

  @override
  void visitTopLevelVariableDeclaration(TopLevelVariableDeclaration node) {
    for (final v in node.variables.variables) {
      _initializer(v);
    }
  }

  @override
  void visitFieldDeclaration(FieldDeclaration node) {
    if (!node.isStatic) return;
    for (final v in node.fields.variables) {
      _initializer(v);
    }
  }

  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    if (node.parent is! CompilationUnit) return;
    final el = node.declaredFragment?.element;
    final fe = node.functionExpression;
    if (el == null || !_hasBody(fe.body)) return;
    _function(
      el,
      node,
      fe.parameters,
      fe.body,
      thisType: null,
    );
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    final el = node.declaredFragment?.element;
    if (el == null || !_hasBody(node.body)) return;
    final cls = el.enclosingElement;
    _function(
      el,
      node,
      node.parameters,
      node.body,
      thisType: node.isStatic ? null : cls is InterfaceElement ? cls.thisType : cls is ExtensionElement ? cls.extendedType : null,
    );
  }

  @override
  void visitConstructorDeclaration(ConstructorDeclaration node) {
    final el = node.declaredFragment?.element;
    if (el == null) return;
    // a factory makes no object of its own: it is a function that returns what its
    // body returns, or (`factory A() = B;`, the shape freezed writes) what the
    // constructor it redirects to makes
    if (node.factoryKeyword != null) {
      final redirect = node.redirectedConstructor?.element;
      if (!_hasBody(node.body) && redirect == null) return;
      _function(el, node, node.parameters, node.body, thisType: null, redirect: redirect);
      return;
    }
    final cls = el.enclosingElement;
    // a constructor with nothing in it is left out, unless it sets fields (its own, or the ones of the
    // classes it extends: the implicit super() runs their initializers)
    if (!prog.ctorsOut.contains(funcId(el))) return;
    _function(el, node, node.parameters, node.body, thisType: null, constructorOf: cls, ctor: node);
  }

  /// A class that declares no constructor has the one that Dart makes: it sets the fields that have an
  /// initializer (`final String v = source();`) and returns the object.
  @override
  void visitClassDeclaration(ClassDeclaration node) {
    final cls = node.declaredFragment?.element;
    final members = node.body is BlockClassBody ? (node.body as BlockClassBody).members : const <ClassMember>[];
    final unnamed = cls?.unnamedConstructor;
    if (cls != null && unnamed != null && !members.any((m) => m is ConstructorDeclaration) && prog.ctorsOut.contains(funcId(unnamed))) {
      _defaultConstructor(cls);
    }
    super.visitClassDeclaration(node);
  }

  void _defaultConstructor(ClassElement cls) {
    final ctor = cls.unnamedConstructor;
    if (ctor == null) return;
    final id = funcId(ctor);
    final out = FuncOut(id)
      ..symbol = symbolOf(ctor)
      ..dep = unit.dep
      ..exported = !(cls.name ?? '').startsWith('_');
    out.pos = unit.pos(unit.unit, 0);
    final f = FnLower(prog, unit, out, cells: const {}, assigned: const {});
    f.lowerInit(
      at: out.pos,
      body: () {
        final d = f.tmp(cls.thisType);
        f.emit('alloc', dsts: [d], at: out.pos, more: {'type': typeId(cls.thisType), 'class': 'pointer'});
        f.bind(thisKey, d);
        // the implicit super(): the object it makes is this one
        var self = d;
        final sup = ctor.superConstructor;
        if (sup != null && prog.ctorsOut.contains(funcId(sup))) {
          self = _implicitSuper(f, sup, out.pos);
          f.bind(thisKey, self);
        }
        _foreignMixinFields(f, cls, out.pos);
        for (final m in _fieldsOf(cls)) {
          final init = m.initializer;
          final fe = m.declaredFragment?.element;
          if (init == null || fe == null) continue;
          final v = f.expr(init);
          f.emit('field_store', args: [self, v], at: f.pos(m), more: {'field': fe.name ?? ''});
        }
        f.ret([self], out.pos);
      },
    );
    prog.out.func(out);
  }

  bool _hasBody(FunctionBody b) => b is! EmptyFunctionBody && b is! NativeFunctionBody;

  void _function(ExecutableElement el, AstNode node, FormalParameterList? list, FunctionBody body,
      {required DartType? thisType, InterfaceElement? constructorOf, ConstructorDeclaration? ctor, ConstructorElement? redirect}) {
    final id = funcId(el);
    final out = FuncOut(id)
      ..symbol = symbolOf(el)
      ..recv = thisType != null
      ..dep = unit.dep
      ..entry = !unit.dep && el is TopLevelFunctionElement && el.name == 'main'
      ..exported = !(el.name ?? '').startsWith('_');
    final at = unit.at(node.offset);
    out.pos = unit.pos(node, node.offset);

    final scan = _Scan();
    // the function itself is the outermost one: its parameters and variables
    // are its own
    scan._stack.add(node);
    node.accept(scan);
    final f = FnLower(prog, unit, out, cells: scan.captured, assigned: scan.assigned);

    final params = <(Object, String, DartType?)>[];
    if (thisType != null) params.add((thisKey, 'this', thisType));
    for (final pe in orderedParams(el)) {
      params.add((pe, pe.name ?? '', pe.type));
    }
    int? self;
    f.lowerBody(
      params: params,
      body: body,
      at: at,
      preamble: redirect != null
          ? () => _redirectFactory(f, redirect, el, out.pos)
          : constructorOf == null
              ? null
              : () {
                  // a constructor makes the object, sets its fields, and returns it
                  final d = f.tmp(constructorOf.thisType);
                  f.emit('alloc', dsts: [d], at: out.pos, more: {'type': typeId(constructorOf.thisType), 'class': 'pointer'});
                  self = d;
                  f.bind(thisKey, d);
                  // a call of the constructor of the superclass (or of another constructor of
                  // the class) makes the object: it is the object of this one too
                  final s = _constructorPrologue(f, constructorOf, ctor!, d, params);
                  if (s != d) {
                    self = s;
                    f.bind(thisKey, s);
                  }
                },
    );
    if (constructorOf != null) _returnSelf(out, self);
    unit.funcTexts(id, node);
    prog.out.func(out);
  }

  /// `factory A(x) = B;`: a call of B with the parameters, which is returned.
  void _redirectFactory(FnLower f, ConstructorElement target, ExecutableElement el, Pos at) {
    final mine = orderedParams(el).toList();
    final theirs = orderedParams(target).toList();
    final args = <int>[];
    for (var i = 0; i < theirs.length; i++) {
      FormalParameterElement? from;
      if (mine.length == theirs.length) {
        from = mine[i];
      } else {
        for (final m in mine) {
          if (m.name == theirs[i].name) from = m;
        }
      }
      args.add(from == null ? f.nilConst(at) : f.readVar(from.baseElement, from.type, at));
    }
    final d = f.tmp(target.enclosingElement.thisType);
    f.emit('call', dsts: [d], args: args, at: at, more: {
      'callee': {'kind': 'static', 'func': funcId(target), 'symbol': symbolOf(target)}
    });
    f.mayThrow();
    f.ret([d], at);
  }

  /// Returns the object the constructor works on: the one it made, or the one that
  /// the constructor it calls (super(...), or this.named(...)) made.
  int _constructorPrologue(FnLower f, InterfaceElement cls, ConstructorDeclaration ctor, int self, List<(Object, String, DartType?)> params) {
    // the constructor that makes the object: super(...) and this.named(...)
    for (final i in ctor.initializers) {
      if (i is SuperConstructorInvocation || i is RedirectingConstructorInvocation) {
        final target = i is SuperConstructorInvocation ? i.element : (i as RedirectingConstructorInvocation).element;
        final list = i is SuperConstructorInvocation ? i.argumentList : (i as RedirectingConstructorInvocation).argumentList;
        if (target == null) {
          f.emit('unsupported', at: f.pos(i), more: {'reason': 'a constructor call that does not resolve: ${i.toSource()}'});
          for (final a in list.arguments) {
            f.expr(a.argumentExpression);
          }
          break;
        }
        // evaluated first: what it takes cannot depend on the object
        // (`Derived(super.v) : super.named()`: the super parameters fill what the call leaves out)
        self = constructorCall(f, target, list, i, cls.thisType, f.pos(i), passed: i is SuperConstructorInvocation ? _superParams(ctor, target) : null, passedAt: f.pos(i));
        if (i is RedirectingConstructorInvocation) return self; // the one it redirects to does the rest
        break;
      }
    }
    // `Derived(super.v)`: the parameters that are passed on to the constructor of the superclass
    if (!ctor.initializers.any((i) => i is SuperConstructorInvocation || i is RedirectingConstructorInvocation)) {
      final sup = ctor.declaredFragment?.element.superConstructor;
      final passed = sup == null ? const <FormalParameterElement, FormalParameterElement>{} : _superParams(ctor, sup);
      if (sup != null && (passed.isNotEmpty || prog.ctorsOut.contains(funcId(sup)))) {
        final args = <int>[];
        for (final tp in orderedParams(sup)) {
          final mine = passed[tp.baseElement];
          args.add(mine == null ? f.nilConst(f.pos(ctor)) : f.readVar(mine.baseElement, mine.type, f.pos(ctor)));
        }
        final r = f.tmp(sup.enclosingElement.thisType);
        f.emit('call', dsts: [r], args: args, at: f.pos(ctor), more: {
          'callee': {'kind': 'static', 'func': funcId(sup), 'symbol': symbolOf(sup)}
        });
        f.mayThrow();
        self = r;
      }
    }
    // fields with an initializer in the class
    _foreignMixinFields(f, cls, f.pos(ctor));
    for (final m in _fieldsOf(cls)) {
      final init = m.initializer;
      final el = m.declaredFragment?.element;
      if (init == null || el == null) continue;
      final v = f.expr(init);
      f.emit('field_store', args: [self, v], at: f.pos(m), more: {'field': el.name ?? ''});
    }
    // this.x parameters
    for (final fp in ctor.parameters.parameters) {
      final x = fp;
      if (x is FieldFormalParameter) {
        final pe = x.declaredFragment?.element;
        if (pe == null) continue;
        final v = f.readVar(pe.baseElement, pe.type, f.pos(x));
        f.emit('field_store', args: [self, v], at: f.pos(x), more: {'field': x.name.lexeme});
      }
    }
    // the initializer list
    for (final i in ctor.initializers) {
      if (i is ConstructorFieldInitializer) {
        final v = f.expr(i.expression);
        f.emit('field_store', args: [self, v], at: f.pos(i), more: {'field': i.fieldName.name});
      } else if (i is AssertInitializer) {
        // the condition and the message are code that runs, as in an assert statement
        f.expr(i.condition);
        final m = i.message;
        if (m != null) f.expr(m);
      } else if (i is! SuperConstructorInvocation && i is! RedirectingConstructorInvocation) {
        f.emit('unsupported', at: f.pos(i), more: {'reason': 'initializer ${i.runtimeType}'});
      }
    }
    return self;
  }

  /// `Derived(super.v)`: the parameter of the super constructor -> the parameter here that is passed on to it.
  Map<FormalParameterElement, FormalParameterElement> _superParams(ConstructorDeclaration ctor, ConstructorElement target) {
    final passed = <FormalParameterElement, FormalParameterElement>{};
    for (final fp in ctor.parameters.parameters) {
      if (fp is SuperFormalParameter) {
        final pe = fp.declaredFragment?.element;
        if (pe is SuperFormalParameterElement) {
          final t = pe.superConstructorParameter;
          if (t != null) passed[t.baseElement] = pe;
        }
      }
    }
    return passed;
  }

  /// The call of the constructor of the superclass that Dart makes when none is written.
  int _implicitSuper(FnLower f, ConstructorElement sup, Pos at) {
    final args = [for (final _ in orderedParams(sup)) f.nilConst(at)];
    final r = f.tmp(sup.enclosingElement.thisType);
    f.emit('call', dsts: [r], args: args, at: at, more: {
      'callee': {'kind': 'static', 'func': funcId(sup), 'symbol': symbolOf(sup)}
    });
    f.mayThrow();
    return r;
  }

  /// The fields that a constructor of [cls] sets from their initializers: the ones of the mixins that
  /// the class applies (in the order of the `with` clause), then its own. What is found is in this unit.
  Iterable<VariableDeclaration> _fieldsOf(InterfaceElement cls) sync* {
    if (cls is ClassElement) {
      for (final m in cls.mixins) {
        yield* _declaredFields(m.element);
      }
    }
    yield* _declaredFields(cls);
  }

  Iterable<VariableDeclaration> _declaredFields(InterfaceElement cls) sync* {
    for (final d in unit.unit.declarations) {
      List<ClassMember>? members;
      if (d is ClassDeclaration && d.declaredFragment?.element == cls) {
        members = d.body is BlockClassBody ? (d.body as BlockClassBody).members : const <ClassMember>[];
      } else if (d is MixinDeclaration && d.declaredFragment?.element == cls) {
        members = d.body.members;
      } else if (d is EnumDeclaration && d.declaredFragment?.element == cls) {
        members = d.body.members;
      }
      for (final m in members ?? const <ClassMember>[]) {
        if (m is FieldDeclaration && !m.isStatic) yield* m.fields.variables;
      }
    }
  }

  /// A mixin of another file has field initializers that this constructor runs, and they are not lowered
  /// here: said, so that nothing is left out without a trace.
  void _foreignMixinFields(FnLower f, InterfaceElement cls, Pos at) {
    if (cls is! ClassElement) return;
    for (final m in cls.mixins) {
      final el = m.element;
      if (!el.fields.any((x) => !x.isStatic && x.hasInitializer)) continue;
      if (_declaredFields(el).isEmpty) {
        f.emit('unsupported', at: at, more: {'reason': 'the field initializers of the mixin ${el.name} are in another file'});
      }
    }
  }

  /// The last block of a constructor returns the object: every `return` of
  /// the lowered body returned nothing.
  void _returnSelf(FuncOut out, int? self) {
    if (self == null) return;
    for (final b in out.blocks) {
      final t = b.term;
      if (t != null && t['kind'] == 'return') {
        t['values'] = [self];
      }
    }
  }
}
