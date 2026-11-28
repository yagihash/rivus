// The driver: resolves the code, lowers every function with a body and writes
// the stream.
import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context.dart';
import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';
import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

import 'irjson.dart';
import 'lower.dart';
import 'names.dart';
import 'program.dart';

/// Extracts the Dart code under [root] (the files that are not under a hidden
/// or a build directory) and writes the stream to [sink].
Future<void> extract(String root, StringSink sink,
    {bool deps = true,
    bool includeTests = false,
    List<String> allowDepsUnder = const [],
    List<String> bodyReplaced = const [],
    List<String> depDirs = const [],
    Future<void> Function()? drain,
    String? failIn}) async {
  root = p.normalize(p.absolute(root));
  // --dep-dir: directories of the project whose files are read as dependencies (their functions are marked dep, as those
  // of the pub cache are), though they are under the project: the skeletons of the packages that the zoo stands in for
  final depRoots = [for (final d in depDirs) p.normalize(p.isAbsolute(d) ? d : p.join(root, d))];
  bool inDepDir(String path) => depRoots.any((d) => p.equals(d, path) || p.isWithin(d, path));
  final out = StreamOut(sink);
  final prog = Program(root, out)..failIn = failIn;
  final collection = AnalysisContextCollection(includedPaths: [root], sdkPath: findSdk());

  // Resolve everything first: the call graph needs all of the classes.
  final units = <UnitInfo>[];
  // The files of the project: the ones the analysis takes, and the ones that
  // analysis_options.yaml leaves out of it (the generated code: *.g.dart and
  // *.freezed.dart are the usual `exclude`), which are code of the project all the same.
  final paths = <String>{};
  // The files of a package that its build does not ship (test/, integration_test/, benchmark/, tool/,
  // example/) are not read unless --include-tests asks (ADR-062). A production file that imports one reads
  // it as a dependency (below), so a flow through it is not lost.
  final testFiles = <String>{};
  final pkgCache = <String, bool>{};
  void take(String path) {
    if (!includeTests && _isNonProduction(root, path, pkgCache)) {
      testFiles.add(path);
      return;
    }
    paths.add(path);
  }
  for (final context in collection.contexts) {
    for (final path in context.contextRoot.analyzedFiles()) {
      if (path.endsWith('.dart') && p.isWithin(root, path) && !_skipped(root, path) && !inDepDir(path)) take(path);
    }
  }
  // the directories of the project with code that is not read because of where they are (said at the end)
  final skippedDirs = <String>{};
  for (final e in _walk(root, out)) {
    if (e is! File || !e.path.endsWith('.dart')) continue;
    if (inDepDir(e.path)) continue; // read as a dependency, when the code imports it (below)
    if (_skipped(root, e.path)) {
      skippedDirs.add(_skippedDir(root, e.path));
    } else {
      take(p.normalize(e.path));
    }
  }
  for (final path in paths.toList()..sort()) {
    final context = _contextOf(collection, path);
    if (context == null) {
      // not in any package of the project: the file is not read, and that is said (what input does this: not known;
      // a tree of packages with a directory that has no pubspec.yaml was read, so there is no fixture)
      out.loadError(p.relative(path, from: root), 'the file is not in any package of the project and was not read', unread: true);
      continue;
    }
    final r = await context.currentSession.getResolvedUnit(path);
    if (r is! ResolvedUnitResult) {
      out.loadError(p.relative(path, from: root), 'could not be resolved');
      continue;
    }
    String? firstSyntax;
    var syntax = 0;
    for (final d in r.diagnostics) {
      if (d.diagnosticCode.type.name == 'COMPILE_TIME_ERROR') {
        out.loadError(p.relative(path, from: root), d.message, func: _enclosingFunction(r.unit, d.offset));
      } else if (d.diagnosticCode.type.name == 'SYNTACTIC_ERROR') {
        firstSyntax ??= d.message;
        syntax++;
      }
    }
    // A file with a syntax error is read on the tree that the analyzer recovers: a declaration after the error may be
    // missing, or made up. It is said (summary.unread; the findings of its functions have a note of it).
    if (firstSyntax != null) {
      // (the core takes the file by this prefix: ir.SyntaxErrorPrefix, held by cmd/rivus/dart_syntax_test.go)
      out.loadError(p.relative(path, from: root),
          'syntax_error: $firstSyntax ($syntax in the file); the file was read on the syntax tree that the analyzer recovered: a declaration may be missing or made up',
          unread: true);
    }
    units.add(UnitInfo(prog, p.relative(path, from: root), r.unit, r.content, r.lineInfo));
  }
  // The dependencies (not the SDK): the libraries the code imports, and what
  // they import. Their functions are marked dep; the core keeps the ones the
  // code reaches.
  final readPackages = <String>{}; // the packages of the dependencies that were reached
  final usedByProduction = <String>{}; // the packages that the code of the project imports directly
  final replacedLibs = <LibraryElement>[]; // the libraries of the packages that are not read: indexed from their elements
  final notRead = <String, int>{}; // package:name -> how many functions it declares, of the packages that were not read
  final refused = <String, String>{}; // the directory of a dependency that was not read -> a file in it
  if (deps) {
    final seen = <String>{};
    final files = <String>[];
    // the context that resolved the library which imports a file of a dependency: that file is resolved in it too,
    // because only its package_config.json says where the dependency is (the first one of the collection need not have it)
    final importedIn = <String, AnalysisContext>{};
    AnalysisContext? visiting;
    // --body-replaced: the packages whose models stand for the code. Their libraries are built (the declarations
    // and the types are needed), but their files are not resolved and not lowered: the program has no function of
    // them, as it has none after the filter of the core (docs/DESIGN-package-models.md 2).
    final replaced = _replacedPackages(bodyReplaced);
    final replacedFiles = <String>{};
    void visit(LibraryElement lib) {
      if (lib.uri.scheme == 'dart' || !seen.add('${lib.uri} ${lib.firstFragment.source.fullName}')) return;
      final isReplaced = lib.uri.scheme == 'package' && replaced.contains(lib.uri.pathSegments.first) && !p.isWithin(root, lib.firstFragment.source.fullName);
      if (isReplaced) replacedLibs.add(lib);
      if (isReplaced) notRead.update('package:${lib.uri.pathSegments.first}', (n) => n + _declaredFunctions(lib), ifAbsent: () => _declaredFunctions(lib));
      for (final f in lib.fragments) {
        final path = f.source.fullName;
        if (isReplaced) replacedFiles.add(path);
        // a file of the project that was left out above (a hidden directory such as `.dart_tool`,
        // the `build` at the top) is read as a dependency when the code imports it
        if (!p.isWithin(root, path) || _skipped(root, path) || inDepDir(path) || testFiles.contains(path)) {
          files.add(path);
          if (visiting != null) importedIn.putIfAbsent(path, () => visiting!);
        }
      }
      if (lib.uri.scheme == 'package') readPackages.add(lib.uri.pathSegments.first);
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
      visiting = context;
      for (final u in List.of(units)) {
        // a file is resolved in the context of its own package, not in the ones of the others
        if (!identical(_contextOf(collection, p.join(root, u.file)), context)) continue;
        final r = await context.currentSession.getResolvedUnit(p.join(root, u.file));
        if (r is ResolvedUnitResult) {
          for (final i in r.libraryElement.firstFragment.libraryImports) {
            final u = i.importedLibrary?.uri;
            if (u != null && u.scheme == 'package') usedByProduction.add(u.pathSegments.first);
          }
          visit(r.libraryElement);
        }
      }
    }
    final done = <String>{};
    final packageDirOf = <String, String>{}; // the name of a dependency -> the directory that was read for it
    final notSaid = <String>{};
    final trusted = _trustedDependencyRoots(allowDepsUnder);
    for (final path in files) {
      if (!path.endsWith('.dart') || !done.add(path)) continue;
      if (replacedFiles.contains(path)) continue; // not resolved: the models stand for it
      // Where a dependency is, is said by the package_config.json of the project, which is not trusted: it can
      // point anywhere. What is read is under the project, the pub cache, the SDKs, or what is asked for.
      if (!p.isWithin(root, path) && !trusted.any((r) => p.equals(r, path) || p.isWithin(r, path))) {
        refused.putIfAbsent(_packageDir(path), () => path);
        continue;
      }
      final context = (p.isWithin(root, path) ? _contextOf(collection, path) : null) ?? importedIn[path] ?? collection.contexts.first;
      final r = await context.currentSession.getResolvedUnit(path);
      if (r is! ResolvedUnitResult) {
        // (what input does this: not known; no fixture)
        out.loadError(p.isWithin(root, path) ? p.relative(path, from: root) : path, 'a file of a dependency could not be resolved and was not read', unread: true);
        continue;
      }
      if (!p.isWithin(root, path)) {
        final uri = r.libraryElement.uri;
        final dir = _packageDir(path);
        // A file of a dependency that is not named by a package: URI cannot be matched with the calls to it (they are
        // named by package: URIs): it is not analysed, and that is said (once for a package).
        if (uri.scheme == 'file') {
          if (notSaid.add(dir)) {
            out.loadError(dir, '$_depNotLinked: the file of a dependency is not resolved to a package: URI (its package_config.json does not name it), so the calls to it do not reach it and its functions are not analysed; point -C at one package, or run `dart pub get` in each package', unread: true);
          }
          continue;
        }
        // Two packages of the tree that read different versions of one dependency: the functions have the same IDs, so
        // only the first is read, and the others are said.
        if (uri.scheme == 'package') {
          final name = uri.pathSegments.first;
          final first = packageDirOf.putIfAbsent(name, () => dir);
          if (first != dir) {
            if (notSaid.add(dir)) {
              out.loadError(dir, '$_depNotLinked: $name is read from another directory by another package of the tree, and the functions of the two have the same IDs: the one in $first is read and analysed (the calls of every package reach it), and this one is not; use the same version of $name in the packages, or point -C at one package', unread: true);
            }
            continue;
          }
        }
      }
      // ADR-114: a package of the project that lies outside -C (a path: dependency, a member of a workspace) is
      // project code when its directory is under -allow-deps-under: its sites are listed.
      final project = !p.isWithin(root, path) && _isAllowedProjectPath(path, allowDepsUnder);
      units.add(UnitInfo(prog, p.isWithin(root, path) ? p.relative(path, from: root) : path, r.unit, r.content, r.lineInfo, dep: !project));
    }
  }
  for (final n in notRead.entries.toList()..sort((a, b) => a.key.compareTo(b.key))) {
    out.bodyNotRead(n.key, n.value);
  }
  for (final e in refused.entries) {
    out.loadError(e.key,
        '$_depNotAllowed: a dependency outside the project, the pub cache and the SDKs is not read (what package_config.json points to is not trusted): --allow-deps-under ${_advisedDir(e.key)} reads it',
        unread: true);
  }
  _missingPackageConfigs(root, out);
  if (!includeTests) {
    final dev = _devOnlyDependencies(root, out);
    final unread = deps ? dev.where((d) => !readPackages.contains(d)).toList() : <String>[];
    final used = deps ? (dev.toList()..retainWhere(usedByProduction.contains)) : <String>[];
    out.testsSkipped(testFiles.length, unread.length, unread..sort(), [
      if (used.isNotEmpty)
        'the code of the project uses a dependency that is declared for development (${(used..sort()).join(', ')}): it is read, because it is imported'
    ], skippedDirs: skippedDirs.length, skippedDirNames: (skippedDirs.toList()..sort()).take(5).toList());
  } else if (skippedDirs.isNotEmpty) {
    out.testsSkipped(0, 0, const [], const [], skippedDirs: skippedDirs.length, skippedDirNames: (skippedDirs.toList()..sort()).take(5).toList());
  }
  units.sort((a, b) => a.file.compareTo(b.file));
  for (final u in units) {
    u.unit.accept(_Index(prog));
  }
  replacedLibs.sort((a, b) => a.uri.toString().compareTo(b.uri.toString()));
  for (final lib in replacedLibs) {
    _Index(prog).library(lib);
  }
  prog.closeCtors();
  for (final u in units) {
    u.unit.accept(_Lower(prog, u));
    // what is written so far goes out: the stream is not kept whole in memory
    if (drain != null) await drain();
  }
  for (final e in prog.globals.entries) {
    out.global(e.key, e.value.$1, e.value.$2);
  }
  for (final id in prog.types.keys.toList()..sort()) {
    out.type(id, prog.types[id]!.toList()..sort());
  }
  prog.texts.write(out);
}

/// The names of the packages that --body-replaced names (`package:name`, or `package:name/...`: the package is the unit, so
/// the ending says nothing more).
Set<String> _replacedPackages(List<String> declared) {
  final out = <String>{};
  for (var d in declared) {
    if (d.endsWith('/...')) d = d.substring(0, d.length - 4);
    if (d.startsWith('package:') && !d.contains('/')) out.add(d.substring('package:'.length));
  }
  return out;
}

/// How many functions the library declares: the top-level ones and the accessors, and the methods, constructors and
/// accessors of its types and extensions. What the report says was left without a body.
int _declaredFunctions(LibraryElement lib) {
  var n = lib.topLevelFunctions.length + lib.getters.length + lib.setters.length;
  for (final t in <InterfaceElement>[...lib.classes, ...lib.enums, ...lib.mixins, ...lib.extensionTypes]) {
    n += t.methods.length + t.constructors.length + t.getters.length + t.setters.length;
  }
  for (final e in lib.extensions) {
    n += e.methods.length + e.getters.length + e.setters.length;
  }
  return n;
}

/// Whether a file is under a directory that was given with --allow-deps-under, and is not in the pub cache, the SDK
/// or Flutter (an allowed directory that holds the cache, such as the home directory, does not make the hosted
/// packages the project's): the project's own packages outside -C (ADR-114).
bool _isAllowedProjectPath(String path, List<String> allow) {
  if (allow.isEmpty) return false;
  final base = _trustedDependencyRoots(const []);
  bool under(String r) => p.equals(r, path) || p.isWithin(r, path);
  if (base.any(under)) return false;
  return allow.map((d) => p.normalize(p.absolute(d))).any(under);
}

/// The directories that a dependency may be read from, besides the project itself: the pub cache, the
/// SDK that runs the extractor (and the Flutter that holds it, whose packages are `package:flutter`),
/// and what the user asked for.
List<String> _trustedDependencyRoots(List<String> extra) {
  final env = Platform.environment;
  final roots = <String>[];
  final home = env['HOME'] ?? env['USERPROFILE'];
  // an empty variable is not set: p.absolute('') is the current directory
  final cache = (env['PUB_CACHE'] != null && env['PUB_CACHE']!.isNotEmpty ? env['PUB_CACHE'] : null) ??
      (Platform.isWindows ? (env['LOCALAPPDATA'] == null ? null : p.join(env['LOCALAPPDATA']!, 'Pub', 'Cache')) : (home == null ? null : p.join(home, '.pub-cache')));
  if (cache != null) roots.add(p.normalize(p.absolute(cache)));
  // The SDK is two above the executable (<sdk>/bin/dart) only when that is an SDK: a compiled extractor in
  // ~/bin would make the whole home directory trusted (yagihash/rivus #61).
  final sdk = findSdk();
  if (sdk != null) {
    roots.add(sdk);
    // <flutter>/bin/cache/dart-sdk
    if (p.basename(sdk) == 'dart-sdk' && p.basename(p.dirname(sdk)) == 'cache') roots.add(p.dirname(p.dirname(p.dirname(sdk))));
  }
  final flutter = env['FLUTTER_ROOT'];
  if (flutter != null && flutter.isNotEmpty) roots.add(p.normalize(p.absolute(flutter)));
  roots.addAll(extra.map((d) => p.normalize(p.absolute(d))));
  return roots;
}

/// The Dart SDK that the analysis reads `dart:*` from, or null (then the analyzer looks for it above the executable). In this
/// order: RIVUS_DART_SDK (named by the user); the SDK that the executable is in (<sdk>/bin/dart: `dart run`, or the
/// extractor compiled and put in the SDK); the SDK of the `dart` that PATH finds (a compiled extractor, which is
/// anywhere: `dart compile exe`, then `brew install`). Only a directory that is an SDK is taken: a compiled extractor in
/// ~/bin would make the whole home directory trusted (yagihash/rivus #61).
String? findSdk({Map<String, String>? environment, String? executable}) {
  final env = environment ?? Platform.environment;
  final named = env['RIVUS_DART_SDK'];
  if (named != null && named.isNotEmpty) {
    final d = p.normalize(p.absolute(named));
    return _isSdk(d) ? d : null;
  }
  final own = p.dirname(p.dirname(executable ?? Platform.resolvedExecutable));
  if (_isSdk(own)) return own;
  final exe = Platform.isWindows ? 'dart.exe' : 'dart';
  for (final dir in (env['PATH'] ?? '').split(Platform.isWindows ? ';' : ':')) {
    if (dir.isEmpty) continue;
    final f = File(p.join(dir, exe));
    if (!f.existsSync()) continue;
    String real;
    try {
      real = f.resolveSymbolicLinksSync();
    } on FileSystemException {
      continue;
    }
    final sdk = p.dirname(p.dirname(real));
    if (_isSdk(sdk)) return sdk;
  }
  return null;
}

bool _isSdk(String dir) =>
    File(p.join(dir, 'version')).existsSync() && Directory(p.join(dir, 'lib', '_internal')).existsSync();

/// The directory that a refusal advises to allow: the one above the package, so that its siblings are read
/// too; but not a directory that is the root or the home, which would open everything: the package itself.
String _advisedDir(String packageDir) {
  final parent = p.dirname(packageDir);
  final home = Platform.environment['HOME'] ?? Platform.environment['USERPROFILE'];
  final shallow = parent == packageDir || p.dirname(parent) == parent || (home != null && p.equals(parent, home));
  return shallow ? packageDir : parent;
}

/// The start of the messages about a dependency that is not read as the calls to it name it (the core and the tests take it by this).
const _depNotLinked = 'dependency_not_linked';

/// The start of the message about a dependency that is outside the places that are trusted (the project, the pub cache, the SDKs, --allow-deps-under): it is not read.
const _depNotAllowed = 'dependency_not_allowed';

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

/// The directory of the project that makes [path] skipped: the path up to the first hidden directory, or `build` at the top.
String _skippedDir(String root, String path) {
  final parts = p.split(p.relative(path, from: root));
  final at = parts.indexWhere((s) => s.startsWith('.'));
  if (at < 0) return parts.first; // `build` at the top
  return p.joinAll(parts.sublist(0, at + 1 > parts.length - 1 ? parts.length - 1 : at + 1));
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
    _noteBody(el);
  }

  void _noteBody(Element el) {
    final base = el.baseElement;
    final id = funcId(base);
    prog.bodies.add(id);
    final cls = base.enclosingElement;
    if (cls is InterfaceElement && base is! ConstructorElement) {
      (prog.methods[memberKey(base)] ??= []).add(MethodImpl(cls, id));
    }
  }

  /// A library whose files are not resolved (--body-replaced): the same index, made from its elements. The program has
  /// no function of it, but what the code of the project asks of it is still answered the way it is when the files are
  /// read: the class hierarchy (the rules ask for the subtypes of its classes), the methods that a call through an
  /// interface may reach (the candidates, which name the functions that are left out), the operators that have a body,
  /// and the constructors that have something to do. What the elements do not say (whether a constructor that has no
  /// parameter has anything in its body or its initializers) is taken to be nothing.
  void library(LibraryElement lib) {
    bool body(ExecutableElement e) =>
        (e is MethodElement ? e.isOriginDeclaration : e is PropertyAccessorElement ? e.isOriginDeclaration : e is TopLevelFunctionElement) &&
        !e.isAbstract &&
        !e.isExternal;
    for (final t in <InterfaceElement>[...lib.classes, ...lib.enums, ...lib.mixins, ...lib.extensionTypes]) {
      _type(t);
      for (final m in <ExecutableElement>[...t.methods, ...t.getters, ...t.setters]) {
        if (body(m)) _noteBody(m);
      }
      if (t is ClassElement && !t.isMixinApplication) {
        final explicit = t.constructors.where((c) => c.isOriginDeclaration).toList();
        if (explicit.isEmpty) {
          final ctor = t.unnamedConstructor;
          if (ctor != null) _noteCtor(ctor, own: _hasFieldInit(t), calls: ctor.superConstructor);
        }
        for (final c in explicit) {
          if (c.isFactory) continue;
          _noteCtor(c, own: c.formalParameters.isNotEmpty || _hasFieldInit(t), calls: c.redirectedConstructor ?? c.superConstructor);
        }
      }
    }
    for (final e in lib.extensions) {
      for (final m in <ExecutableElement>[...e.methods, ...e.getters, ...e.setters]) {
        if (body(m)) _noteBody(m);
      }
    }
    for (final f in <ExecutableElement>[...lib.topLevelFunctions, ...lib.getters, ...lib.setters]) {
      if (body(f)) _noteBody(f);
    }
  }

  /// What the class is a subtype of, directly: the rules ask for the
  /// subclasses of a framework's class (the services of grpc).
  void _type(InterfaceElement? el) {
    if (el == null) return;
    prog.classes.add(el);
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

  /// A declaration whose element the analyzer could not make is left out (there is nothing to lower it with; what
  /// input does this other than a compile error: not known; no fixture): said,
  /// with where it is, so that what is missing is not missing without a trace (the file is in summary.unread).
  void _noElement(AstNode node, String what) {
    final src = node.toSource().replaceAll(RegExp(r'\s+'), ' ');
    prog.out.loadError(unit.file,
        'a $what whose element the analyzer could not make is left out (line ${unit.lines.getLocation(node.offset).lineNumber}): ${src.length > 60 ? src.substring(0, 60) : src}',
        unread: true);
  }

  /// The initializer of a variable of the library (or a static field) is a function of its
  /// own, which stores the value in the variable: the program starts there too (the
  /// variable is set when it is first read, and the code that reads it does not ask when).
  void _initializer(VariableDeclaration v) {
    final el = v.declaredFragment?.element;
    if (v.initializer != null && el == null) {
      _noElement(v, 'variable initializer');
      return;
    }
    if (v.initializer == null || el == null) return;
    _guarded('${symbolOf(el)}\$init', () => _initializerLower(v));
  }

  void _initializerLower(VariableDeclaration v) {
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
    if (el == null) {
      _noElement(node, 'function');
      return;
    }
    if (!_hasBody(fe.body)) return;
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
    if (el == null) {
      _noElement(node, 'method');
      return;
    }
    if (!_hasBody(node.body)) return;
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
    if (el == null) {
      _noElement(node, 'constructor');
      return;
    }
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
    _guarded(funcId(ctor), () => _defaultConstructorLower(cls));
  }

  void _defaultConstructorLower(ClassElement cls) {
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

  /// A function that cannot be lowered (a null that the code did not expect, an analyzer that answers
  /// otherwise than the extractor was written for) is said in the stream, and costs that function only:
  /// not the whole package (ADR-007: what could not be expressed is recorded, not dropped).
  void _guarded(String id, void Function() lower) {
    try {
      if (prog.failIn != null && id.contains(prog.failIn!)) throw StateError('injected failure in $id');
      lower();
    } catch (e, st) {
      final where = st.toString().split('\n').take(3).join(' | ');
      prog.out.loadError(id, 'the function could not be read by the extractor and is left out: $e ($where)');
    }
  }

  /// The methods of the supertypes that a method overrides or implements (a getter and a setter likewise), by
  /// symbol, outside the SDK: what a framework calls when an app overrides it (`State.initState`,
  /// `StatelessWidget.build`). The core takes a method that overrides what the code that was read does not
  /// declare as a function the framework calls (`Func.overrides`). Private names are left out (a library's
  /// own); so are the members of `dart:*` (`Object.toString`, `Iterator.moveNext` are called by anyone).
  List<String> _overrides(ExecutableElement el) {
    final base = el.baseElement;
    final cls = base.enclosingElement;
    final raw = base.name ?? '';
    final name = base is SetterElement && raw.endsWith('=') ? raw.substring(0, raw.length - 1) : raw;
    if (cls is! InterfaceElement || base is ConstructorElement || base.isStatic || name.isEmpty || name.startsWith('_')) {
      return const [];
    }
    final out = <String>{};
    for (final s in cls.allSupertypes) {
      final e = s.element;
      final Element? m = base is SetterElement
          ? e.getSetter(name)
          : base is GetterElement
              ? e.getGetter(name)
              : e.getMethod(name);
      if (m == null || m.isPrivate || m.library?.uri.scheme == 'dart') continue;
      out.add(symbolOf(m));
    }
    return out.toList()..sort();
  }

  void _function(ExecutableElement el, AstNode node, FormalParameterList? list, FunctionBody body,
          {required DartType? thisType, InterfaceElement? constructorOf, ConstructorDeclaration? ctor, ConstructorElement? redirect}) =>
      _guarded(funcId(el), () => _functionLower(el, node, list, body, thisType: thisType, constructorOf: constructorOf, ctor: ctor, redirect: redirect));

  void _functionLower(ExecutableElement el, AstNode node, FormalParameterList? list, FunctionBody body,
      {required DartType? thisType, InterfaceElement? constructorOf, ConstructorDeclaration? ctor, ConstructorElement? redirect}) {
    final id = funcId(el);
    final out = FuncOut(id)
      ..symbol = symbolOf(el)
      ..recv = thisType != null
      ..dep = unit.dep
      ..entry = !unit.dep && el is TopLevelFunctionElement && el.name == 'main'
      ..overrides = _overrides(el)
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

const _nonProductionDirs = {'test', 'integration_test', 'benchmark', 'tool', 'example'};

/// Whether the file is in a directory of a package that the package does not ship: the first directory
/// under the package (the one with a pubspec.yaml; the project itself when it has none) is one of
/// [_nonProductionDirs]. `lib/src/test/` and `lib/foo_test.dart` are code of the library.
bool _isNonProduction(String root, String path, Map<String, bool> pkgCache) {
  var dir = p.dirname(path);
  while (true) {
    final isPkg = pkgCache.putIfAbsent(dir, () => File(p.join(dir, 'pubspec.yaml')).existsSync());
    if (isPkg || p.equals(dir, root)) {
      final parts = p.split(p.relative(path, from: dir));
      if (parts.length > 1 && _nonProductionDirs.contains(parts.first)) return true;
    }
    if (p.equals(dir, root) || !p.isWithin(root, dir)) return false;
    dir = p.dirname(dir);
  }
}

/// Every file and directory under [root] (the links are not followed). A directory that cannot be read (no permission, or it
/// went away) does not stop the walk: it is said as a load error (when [out] is given: the first walk says it, the later ones
/// pass null) and what is under it is not read. (`listSync(recursive: true)` throws at the first one and nothing is written.)
List<FileSystemEntity> _walk(String root, StreamOut? out) {
  final found = <FileSystemEntity>[];
  void visit(Directory dir) {
    final List<FileSystemEntity> entries;
    try {
      entries = dir.listSync(followLinks: false);
    } on FileSystemException catch (err) {
      out?.loadError(p.relative(dir.path, from: root),
          'the directory could not be read (${err.osError?.message ?? err.message}): the Dart files under it are not read',
          unread: true);
      return;
    }
    for (final e in entries) {
      found.add(e);
      if (e is Directory) visit(e);
    }
  }

  visit(Directory(root));
  return found;
}

/// The packages that the pubspec.yaml files under [root] declare under dev_dependencies and not under
/// dependencies (nor as a package of the project itself).
Set<String> _devOnlyDependencies(String root, StreamOut out) {
  final dev = <String>{}, prod = <String>{}, own = <String>{};
  for (final e in _walk(root, null)) {
    if (e is! File || p.basename(e.path) != 'pubspec.yaml' || _skipped(root, e.path)) continue;
    try {
      final y = loadYaml(e.readAsStringSync());
      if (y is! YamlMap) continue;
      if (y['name'] is String) own.add(y['name'] as String);
      final d = y['dev_dependencies'], m = y['dependencies'];
      if (d is YamlMap) dev.addAll(d.keys.cast<String>());
      if (m is YamlMap) prod.addAll(m.keys.cast<String>());
    } catch (err) {
      // the dev-only dependencies are told from the ones that the project ships by these files: not read, said
      out.loadError(p.relative(e.path, from: root), 'pubspec.yaml could not be read ($err): which dependencies are for development only is not known from it');
    }
  }
  return dev.difference(prod).difference(own);
}

/// The ID of the innermost function, method or constructor declaration of [unit] that holds [offset] (a closure
/// counts as the declaration around it), or null when the place is outside of them (an import, a class header).
String? _enclosingFunction(CompilationUnit unit, int offset) {
  String? best;
  var bestLen = 1 << 30;
  void consider(AstNode n, String? id) {
    if (id == null || offset < n.offset || offset > n.end || n.length >= bestLen) return;
    best = id;
    bestLen = n.length;
  }

  unit.accept(_DeclFinder(consider));
  return best;
}

class _DeclFinder extends RecursiveAstVisitor<void> {
  _DeclFinder(this.consider);
  final void Function(AstNode, String?) consider;
  @override
  void visitFunctionDeclaration(FunctionDeclaration node) {
    // a local function is named after the function around it in the IR (`f$1`), not by its own name: the error is in
    // the function around it (a closure counts as it too)
    if (node.parent is CompilationUnit) {
      final el = node.declaredFragment?.element;
      consider(node, el == null ? null : funcId(el));
    }
    super.visitFunctionDeclaration(node);
  }

  @override
  void visitMethodDeclaration(MethodDeclaration node) {
    final el = node.declaredFragment?.element;
    consider(node, el == null ? null : funcId(el));
    super.visitMethodDeclaration(node);
  }

  @override
  void visitConstructorDeclaration(ConstructorDeclaration node) {
    final el = node.declaredFragment?.element;
    consider(node, el == null ? null : funcId(el));
    super.visitConstructorDeclaration(node);
  }

  /// The initializer of a variable of the library or a static field is a function of its own (`name$init`); the one of a
  /// field of an object is lowered in the constructor that the class has by default (`Class.new`).
  @override
  void visitVariableDeclaration(VariableDeclaration node) {
    final el = node.declaredFragment?.element;
    if (node.initializer != null && el != null) {
      if (el is TopLevelVariableElement || (el is FieldElement && el.isStatic)) {
        consider(node, '${symbolOf(el)}\$init');
      } else if (el is FieldElement) {
        final cls = el.enclosingElement;
        if (cls is ClassElement && cls.unnamedConstructor != null) consider(node, funcId(cls.unnamedConstructor!));
      }
    }
    super.visitVariableDeclaration(node);
  }
}

/// A package whose `package_config.json` is not there (`dart pub get` was not run, or failed): its imports of other packages
/// do not resolve, its functions are named by `file:` URIs and not by `package:` ones, and what the rules say about
/// `package:...` does not match: nothing is found, and nothing said. The package config is looked for in the directory of
/// the package and above it (a workspace has one at its root). It is said as a load error that the package was not read as
/// it is, and what to do: rivus does not run `dart pub get` itself (what is fetched is the user's to start).
/// And a package config that is there and old: the pubspec.yaml declares a dependency that the package config does not list
/// (it was added after `dart pub get`): said too, with the names. (Not by the age of the files: a file that was touched
/// and not changed would be called old when it is not.)
void _missingPackageConfigs(String root, StreamOut out) {
  String? configOf(String dir) {
    for (var d = dir;; d = p.dirname(d)) {
      final f = p.join(d, '.dart_tool', 'package_config.json');
      if (File(f).existsSync()) return f;
      if (p.dirname(d) == d) return null;
    }
  }

  for (final e in _walk(root, null)) {
    if (e is! File || p.basename(e.path) != 'pubspec.yaml' || _skipped(root, e.path)) continue;
    final dir = p.dirname(e.path);
    final rel = p.relative(dir, from: root);
    final cfg = configOf(dir);
    if (cfg == null) {
      out.loadError(rel,
          'package_config.json is not there: run `dart pub get` in ${rel == '.' ? 'the directory of the project' : rel} (rivus does not run it). The imports of other packages do not resolve, and the names of the functions are file: ones, so the rules that name package:... do not match: what this package holds is not read as it is',
          unread: true);
      continue;
    }
    try {
      final y = loadYaml(e.readAsStringSync());
      final listed = <String>{};
      final c = jsonDecode(File(cfg).readAsStringSync());
      if (c is Map && c['packages'] is List) {
        for (final pk in c['packages'] as List) {
          if (pk is Map && pk['name'] is String) listed.add(pk['name'] as String);
        }
      }
      final declared = <String>{};
      if (y is YamlMap) {
        for (final k in ['dependencies', 'dev_dependencies']) {
          final d = y[k];
          if (d is YamlMap) declared.addAll(d.keys.cast<String>());
        }
      }
      final missing = (declared.difference(listed).toList())..sort();
      if (missing.isNotEmpty) {
        out.loadError(rel,
            'package_config.json is old: pubspec.yaml declares ${missing.join(', ')}, which it does not list: run `dart pub get` in ${rel == '.' ? 'the directory of the project' : rel} (rivus does not run it). The imports of these packages do not resolve',
            unread: true);
      }
    } catch (_) {
      // a pubspec.yaml or a package config that cannot be read: nothing to compare (the pubspec case is said elsewhere)
    }
  }
}
