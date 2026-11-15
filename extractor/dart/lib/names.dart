// Names and classes of Dart things, as the rules and the IR write them.
//
// A symbol is "<library uri>#<Type>.<member>", for example
// "package:shelf/src/request.dart#Request.url", and "<library uri>#<name>" for
// a top level function. A constructor is "Type.new" or "Type.name".
import 'package:analyzer/dart/element/element.dart';
import 'package:analyzer/dart/element/type.dart';

String libraryUri(Element e) {
  final lib = e.library;
  return lib == null ? '' : lib.uri.toString();
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
  // The setter of a field is named "x=" by the analyzer; the rules speak of
  // the field.
  if (name.endsWith('=')) name = name.substring(0, name.length - 1);
  if (enclosing is InterfaceElement) {
    return '$uri#${_name(enclosing)}.$name';
  }
  if (enclosing is ExtensionElement) {
    return '$uri#${_name(enclosing)}.$name';
  }
  return '$uri#$name';
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

/// The type as a reader writes it.
String displayType(DartType? t) => t?.getDisplayString() ?? '';
