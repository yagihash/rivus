// Names and classes of Dart things, as the rules and the IR write them.
//
// A symbol is "<library uri>#<Type>.<member>", for example
// "package:shelf/src/request.dart#Request.url", and "<library uri>#<name>" for
// a top level function. A constructor is "Type.new" or "Type.name".
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';

/// The libraries of the SDK that others export are named by the library
/// people import: HttpRequest is declared in dart:_http and imported from
/// dart:io.
const _publicLibrary = {
  'dart:_http': 'dart:io',
};

String libraryUri(Element e) {
  final lib = e.library;
  if (lib == null) return '';
  final uri = lib.uri.toString();
  return _publicLibrary[uri] ?? uri;
}

String _name(Element e) => e.name ?? '';

/// The symbol of a function, method, constructor, accessor or variable.
String symbolOf(Element e) {
  final base = e.baseElement;
  final enclosing = base.enclosingElement;
  final uri = libraryUri(base);
  var name = _name(base);
  if (base is ConstructorElement) {
    name = name.isEmpty ? 'new' : name;
  }
  // The unary minus has no parameter and the binary one has one, and the
  // analyzer gives both the name "-": they are two methods (Int32 of fixnum
  // declares both), so the identity tells them apart as Dart's own name does.
  if (base is MethodElement && name == '-' && base.formalParameters.isEmpty) {
    name = 'unary-';
  }
  // A class can have a named constructor and a method of the same name
  // (Utf8BytesScanner.createRecoveryOptionScanner; or a getter): the method gets a mark.
  if ((base is MethodElement || base is GetterElement) && enclosing is InterfaceElement && enclosing.constructors.any((c) => (c.name ?? '') == name)) {
    name = '$name()';
  }
  // The setter of a field is named "x=" by the analyzer; the rules speak of
  // the field. (The operator "[]=" is a method and keeps its name.)
  if (base is SetterElement && name.endsWith('=')) name = name.substring(0, name.length - 1);
  if (enclosing is InterfaceElement) {
    return '$uri#${_name(enclosing)}.$name';
  }
  if (enclosing is ExtensionElement) {
    // an extension without a name: several of them can be in one library,
    // each with a member of the same name; the extended type and the place tell them apart
    final ext = _name(enclosing).isNotEmpty ? _name(enclosing) : '(on ${typeId(enclosing.extendedType)}@${enclosing.firstFragment.offset})';
    return '$uri#$ext.$name';
  }
  return '$uri#$name';
}

/// The identity of a function: its symbol, and "=" for a setter, which the
/// getter of the same name would otherwise share.
String funcId(Element e) {
  final base = e.baseElement;
  return base is SetterElement ? '${symbolOf(base)}=' : symbolOf(base);
}

/// The identity of a type for the IR: "<library uri>#<Name>", without the
/// type arguments.
String typeId(DartType? t) {
  if (t is InterfaceType) {
    return '${libraryUri(t.element)}#${_name(t.element)}';
  }
  if (t is FunctionType) return 'Function';
  if (t is TypeParameterType) return typeId(t.bound);
  if (t == null) return '';
  return 'dynamic';
}

/// How the engine sees a type: a value that stands for itself, or one that
/// refers to storage (an object, a collection).
String typeClass(DartType? t) {
  if (t is InterfaceType) {
    final uri = libraryUri(t.element);
    final name = _name(t.element);
    if (uri == 'dart:core') {
      switch (name) {
        case 'bool':
          return 'bool';
        case 'String':
          return 'string';
        case 'int':
        case 'double':
        case 'num':
          return 'number';
        case 'List':
        case 'Set':
        case 'Map':
        case 'Iterable':
        case 'Iterator':
          return 'container';
        case 'Object':
        case 'Null':
        case 'Type':
        case 'Symbol':
        case 'Enum':
        case 'Record':
          return 'other';
      }
    }
    if (uri == 'dart:async') return 'other'; // a Future or a Stream holds a value that await takes out
    if (uri == 'dart:collection') return 'container';
    if (t.element is EnumElement) return 'other';
    return 'pointer';
  }
  if (t is FunctionType) return 'func';
  if (t is RecordType) return 'struct';
  if (t is TypeParameterType) return 'other';
  return 'other';
}

/// The parameters in the order the IR has them: the positional ones as
/// declared, then the named ones by name. A method and its overrides may
/// declare the named parameters in another order; by name they line up.
List<FormalParameterElement> orderedParams(ExecutableElement el) {
  final all = el.formalParameters.map((p) => p.baseElement).toList();
  final positional = all.where((p) => !p.isNamed).toList();
  final named = all.where((p) => p.isNamed).toList()..sort((a, b) => (a.name ?? '').compareTo(b.name ?? ''));
  return [...positional, ...named];
}

/// The type as a reader writes it.
String displayType(DartType? t) => t?.getDisplayString() ?? '';
