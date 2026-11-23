// The public API of a Dart package, as JSON, for bench/model-author (docs/DESIGN-package-models.md, 6.11).
//
// usage: dart run bin/api.dart <project dir> <package name> [<library prefix>...]
//
// <project dir> is a directory where `dart pub get` ran with the package as a dependency. The package is READ (resolved by
// the analyzer: types only); nothing of it is run. The public libraries are the files under lib/ that are not under lib/src/
// (and, when a prefix is given, only the ones whose path under lib/ starts with it); the API is what they export and the
// package declares. A symbol is written as the extractor writes it (`package:x/src/a.dart#Class.member`: the library that
// declares it), so that it is the symbol of a row of a model.
import 'dart:convert';
import 'dart:io';

import 'package:analyzer/dart/analysis/analysis_context_collection.dart';
import 'package:analyzer/dart/analysis/results.dart';
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';
import 'package:path/path.dart' as p;
import 'package:rivus_dart/names.dart';

Future<void> main(List<String> args) async {
  if (args.length < 2) {
    stderr.writeln('usage: api.dart <project dir> <package> [<lib prefix>...]');
    exit(2);
  }
  final dir = p.normalize(p.absolute(args[0]));
  final pkg = args[1];
  final prefixes = args.sublist(2);
  final libDir = _libDir(dir, pkg);
  if (libDir == null) {
    stderr.writeln('$pkg is not in .dart_tool/package_config.json');
    exit(3);
  }
  final files = <String>[];
  for (final f in Directory(libDir).listSync(recursive: true, followLinks: false)) {
    if (f is! File || !f.path.endsWith('.dart')) continue;
    final rel = p.relative(f.path, from: libDir).split(p.separator).join('/');
    if (rel.startsWith('src/')) continue;
    if (prefixes.isNotEmpty && !prefixes.any(rel.startsWith)) continue;
    files.add(rel);
  }
  files.sort();
  final collection = AnalysisContextCollection(includedPaths: [dir], sdkPath: _sdk());
  final session = collection.contextFor(dir).currentSession;
  final api = _Api(pkg);
  var libs = 0;
  for (final rel in files) {
    final r = await session.getLibraryByUri('package:$pkg/$rel');
    if (r is! LibraryElementResult) continue; // a part, or a file that is not a library
    libs++;
    for (final e in r.element.exportNamespace.definedNames2.values) {
      api.take(e);
    }
  }
  stdout.write(json.encode({'package': pkg, 'libraries': libs, 'symbols': api.list}));
}

String? _sdk() {
  final exe = Platform.resolvedExecutable; // <sdk>/bin/dart
  return p.dirname(p.dirname(exe));
}

String? _libDir(String dir, String pkg) {
  final b = File(p.join(dir, '.dart_tool', 'package_config.json')).readAsStringSync();
  final pc = json.decode(b) as Map<String, dynamic>;
  for (final e in pc['packages'] as List) {
    if (e['name'] != pkg) continue;
    var root = e['rootUri'] as String;
    final u = Uri.parse(root);
    final path = u.scheme == 'file' ? u.toFilePath() : p.normalize(p.join(dir, '.dart_tool', u.toFilePath()));
    return p.normalize(p.join(path, e['packageUri'] as String));
  }
  return null;
}

class _Api {
  _Api(this.pkg);
  final String pkg;
  final seen = <String>{};
  final list = <Map<String, dynamic>>[];

  bool _own(Element e) {
    final uri = e.library?.uri.toString() ?? '';
    return uri.startsWith('package:$pkg/');
  }

  bool _hidden(Element e) {
    final n = e.name ?? '';
    if (n.startsWith('_')) return true;
    return e.metadata.hasInternal || e.metadata.hasVisibleForTesting;
  }

  void take(Element e) {
    if (!_own(e) || _hidden(e)) return;
    if (e is InterfaceElement) {
      _type(e);
    } else if (e is ExtensionElement) {
      _container(e, e.name ?? '', e.fields, e.getters, e.setters, e.methods, const []);
    } else if (e is TopLevelFunctionElement) {
      _exec(e, 'function', '');
    } else if (e is GetterElement && e.isOriginDeclaration) {
      _exec(e, 'getter', '');
    } else if (e is TopLevelVariableElement) {
      _field(e, '');
    }
  }

  void _type(InterfaceElement t) {
    final cls = t.name ?? '';
    final sym = '${libraryUri(t)}#$cls';
    if (!seen.add('type $sym')) return;
    final abstractish = (t is ClassElement && (t.isAbstract || t.isInterface)) || t is MixinElement;
    list.add({
      'symbol': sym,
      'name': cls,
      'kind': 'type',
      'class': cls,
      'abstract': abstractish,
      'fields': [
        for (final f in t.fields)
          if (!(f.name ?? '_').startsWith('_') && !f.isStatic && f.isOriginDeclaration) {'name': f.name, 'type': _typeInfo(f.type)}
      ],
    });
    _container(t, cls, t.fields, t.getters, t.setters, t.methods, t.constructors);
  }

  void _container(Element owner, String cls, List<FieldElement> fields, List<GetterElement> getters, List<SetterElement> setters,
      List<MethodElement> methods, List<ConstructorElement> ctors) {
    for (final c in ctors) {
      if ((c.name ?? '').startsWith('_') || _hidden(c)) continue;
      _exec(c, 'constructor', cls);
    }
    for (final f in fields) {
      if (_hidden(f) || !f.isOriginDeclaration) continue;
      _field(f, cls);
    }
    for (final g in getters) {
      if (_hidden(g) || !g.isOriginDeclaration) continue; // a getter that a field makes is the field
      _exec(g, 'getter', cls);
    }
    for (final s in setters) {
      if (_hidden(s) || !s.isOriginDeclaration) continue;
      _exec(s, 'setter', cls);
    }
    for (final m in methods) {
      if (_hidden(m)) continue;
      _exec(m, 'method', cls);
    }
  }

  void _field(VariableElement f, String cls) {
    final sym = symbolOf(f);
    if (!seen.add('field $sym')) return;
    list.add({
      'symbol': sym,
      'name': f.name ?? '',
      'kind': 'field',
      'class': cls,
      'static': f is FieldElement ? f.isStatic : true,
      'ret': _typeInfo(f.type),
      'params': <Map<String, dynamic>>[],
    });
  }

  void _exec(ExecutableElement e, String kind, String cls) {
    final sym = symbolOf(e);
    if (!seen.add('$kind $sym')) return;
    final params = <Map<String, dynamic>>[];
    var pos = 0;
    for (final p in e.formalParameters) {
      final named = p.isNamed;
      params.add({
        'name': p.name ?? '',
        'named': named,
        'index': named ? -1 : pos,
        'required': p.isRequired,
        'type': _typeInfo(p.type),
      });
      if (!named) pos++;
    }
    final n = e is ConstructorElement ? (e.name == null || e.name == 'new' || e.name!.isEmpty ? 'new' : e.name!) : (e.name ?? '');
    list.add({
      'symbol': sym,
      'name': n,
      'kind': kind,
      'class': cls,
      'static': e.isStatic,
      'ret': _typeInfo(e is ConstructorElement ? null : e.returnType),
      'params': params,
    });
  }

  // How a type is told to the rules: a class of the shape ("string", "uri", "bytes", "strings", "reader", "func", "object", "own
  // abstract class" ...), and the identity of the type (as the IR writes it) for the types of the package.
  Map<String, dynamic> _typeInfo(DartType? t) {
    if (t == null) return {'c': 'void', 't': '', 'id': ''};
    var u = t;
    // Future<T> and FutureOr<T> are what they carry
    if (u is InterfaceType && u.element.library.uri.toString() == 'dart:async' && (u.element.name == 'Future' || u.element.name == 'FutureOr') && u.typeArguments.length == 1) {
      u = u.typeArguments.first;
    }
    final text = t.getDisplayString();
    final id = typeId(u);
    var c = 'other';
    var own = false;
    var abs = false;
    if (u is FunctionType) {
      c = 'func';
    } else if (u is DynamicType) {
      c = 'object';
    } else if (u is InterfaceType) {
      final el = u.element;
      final lib = libraryUri(el);
      final name = el.name ?? '';
      if (lib == 'dart:core' && name == 'String') {
        c = 'string';
      } else if (lib == 'dart:core' && name == 'Uri') {
        c = 'uri';
      } else if (lib == 'dart:core' && name == 'Object') {
        c = 'object';
      } else if (_isBytes(u)) {
        c = 'bytes';
      } else if ((name == 'List' || name == 'Iterable' || name == 'Set') && lib.startsWith('dart:') && u.typeArguments.length == 1 && _isString(u.typeArguments.first)) {
        c = 'strings';
      } else if (name == 'Stream' && lib == 'dart:async' && u.typeArguments.length == 1 && _isBytes(u.typeArguments.first)) {
        c = 'reader';
      } else if (name == 'Map' && lib == 'dart:core') {
        c = 'map';
      } else if (lib.startsWith('package:$pkg/')) {
        c = 'own';
        abs = (el is ClassElement && (el.isAbstract || el.isInterface)) || el is MixinElement;
        own = true;
      }
    }
    return {'c': c, 't': text, 'id': id, 'own': own, 'iface': abs};
  }

  bool _isString(DartType t) => t is InterfaceType && t.element.name == 'String' && libraryUri(t.element) == 'dart:core';

  bool _isBytes(DartType t) {
    if (t is! InterfaceType) return false;
    final n = t.element.name ?? '';
    final lib = libraryUri(t.element);
    if (n == 'Uint8List' && lib == 'dart:typed_data') return true;
    if ((n == 'List' || n == 'Iterable') && lib.startsWith('dart:') && t.typeArguments.length == 1) {
      final a = t.typeArguments.first;
      return a is InterfaceType && a.element.name == 'int' && libraryUri(a.element) == 'dart:core';
    }
    return false;
  }
}
