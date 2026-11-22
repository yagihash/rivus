// Writes a large set of protoc_plugin-shaped files (messages with many fields, nesting, repeated, map,
// oneof and enum fields, and one service that uses them), to measure the analysis on generated code of
// a realistic size. It is not run in CI. The output is not committed (it is a few MB): run it.
//
//   dart run gen/gen.dart --messages 50 --app-dir <app> --pb-dir <dir> --pb-import <uri>
//
// --app-dir   the app package (the service and bin/bulk_server.dart are written there; its name is grpc_bench)
// --pb-dir    where bulk.pb.dart, bulk.pbenum.dart and bulk.pbgrpc.dart go
// --pb-import the URI the service imports bulk.pbgrpc.dart by, e.g. package:grpc_bench/src/generated/bulk.pbgrpc.dart
//             (the generated code in the app package) or package:bulk_protos/bulk.pbgrpc.dart (in a path dependency)
import 'dart:io';

String camel(String s) {
  final p = s.split('_');
  return p[0] + p.skip(1).map((x) => x[0].toUpperCase() + x.substring(1)).join();
}

String cap(String s) {
  final c = camel(s);
  return c[0].toUpperCase() + c.substring(1);
}

const hdr = '''// This is a generated file - do not edit.
//
// Generated from bulk.proto (by gen/gen.dart, in the shape of protoc_plugin's output).

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, unused_import

''';

class F {
  final String name, kind;
  final int tag;
  final String? msg;
  F(this.name, this.kind, this.tag, [this.msg]);
}

List<F> fieldsOf(int i) {
  final fs = <F>[];
  var t = 0;
  F add(String n, String k, [String? m]) {
    final f = F(n, k, ++t, m);
    fs.add(f);
    return f;
  }

  add('name', 'string');
  add('id', 'int32');
  add('big', 'int64');
  add('flag', 'bool');
  add('score', 'double');
  add('data', 'bytes');
  add('tags', 'rstring');
  add('labels', 'map');
  add('status', 'enum');
  if (i > 0) {
    add('child', 'message', 'Msg${i - 1}');
    add('children', 'rmessage', 'Msg${i - 1}');
  }
  for (var k = 0; k < 3 * (i % 5); k++) {
    add('f$k', 'string');
  }
  return fs;
}

String message(int i) {
  final n = 'Msg$i';
  final fs = fieldsOf(i);
  final oneofTags = [fs.length + 1, fs.length + 2];
  final b = StringBuffer();
  b.writeln('class $n extends \$pb.GeneratedMessage {');
  b.writeln('  factory $n({');
  for (final f in fs) {
    b.writeln('    ${ctor(f)} ${camel(f.name)},');
  }
  b.writeln('    \$core.String? text,');
  b.writeln('    \$core.int? number,');
  b.writeln('  }) {\n    final result = create();');
  for (final f in fs) {
    final c = camel(f.name);
    if (f.kind == 'rstring' || f.kind == 'rmessage') {
      b.writeln('    if ($c != null) result.$c.addAll($c);');
    } else if (f.kind == 'map') {
      b.writeln('    if ($c != null) result.$c.addEntries($c);');
    } else {
      b.writeln('    if ($c != null) result.$c = $c;');
    }
  }
  b.writeln('    if (text != null) result.text = text;');
  b.writeln('    if (number != null) result.number = number;');
  b.writeln('    return result;\n  }\n\n  $n._();\n');
  b.writeln('''  factory $n.fromBuffer(\$core.List<\$core.int> data,
          [\$pb.ExtensionRegistry registry = \$pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory $n.fromJson(\$core.String json,
          [\$pb.ExtensionRegistry registry = \$pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);
''');
  b.writeln('''  static const \$core.Map<\$core.int, ${n}_Choice> _${n}_ChoiceByTag = {
    ${oneofTags[0]}: ${n}_Choice.text,
    ${oneofTags[1]}: ${n}_Choice.number,
    0: ${n}_Choice.notSet
  };
  static final \$pb.BuilderInfo _i = \$pb.BuilderInfo(
      _omitMessageNames ? '' : '$n',
      package: const \$pb.PackageName(_omitMessageNames ? '' : 'bench.bulk'),
      createEmptyInstance: create)
    ..oo(0, [${oneofTags[0]}, ${oneofTags[1]}])''');
  for (final f in fs) {
    b.writeln('    ${info(f, n)}');
  }
  b.writeln('''    ..aOS(${oneofTags[0]}, _omitFieldNames ? '' : 'text')
    ..aI(${oneofTags[1]}, _omitFieldNames ? '' : 'number')
    ..hasRequiredFields = false;

  @\$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  $n clone() => deepCopy();
  @\$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  $n copyWith(void Function($n) updates) =>
      super.copyWith((message) => updates(message as $n)) as $n;

  @\$core.override
  \$pb.BuilderInfo get info_ => _i;

  @\$core.pragma('dart2js:noInline')
  static $n create() => $n._();
  @\$core.override
  $n createEmptyInstance() => create();
  @\$core.pragma('dart2js:noInline')
  static $n getDefault() =>
      _defaultInstance ??= \$pb.GeneratedMessage.\$_defaultFor<$n>(create);
  static $n? _defaultInstance;

  @\$pb.TagNumber(${oneofTags[0]})
  @\$pb.TagNumber(${oneofTags[1]})
  ${n}_Choice whichChoice() => _${n}_ChoiceByTag[\$_whichOneof(0)]!;
  @\$pb.TagNumber(${oneofTags[0]})
  @\$pb.TagNumber(${oneofTags[1]})
  void clearChoice() => \$_clearField(\$_whichOneof(0));
''');
  var idx = 0;
  for (final f in fs) {
    b.write(accessors(f, idx++));
  }
  b.writeln('''
  @\$pb.TagNumber(${oneofTags[0]})
  \$core.String get text => \$_getSZ($idx);
  @\$pb.TagNumber(${oneofTags[0]})
  set text(\$core.String value) => \$_setString($idx, value);
  @\$pb.TagNumber(${oneofTags[0]})
  \$core.bool hasText() => \$_has($idx);
  @\$pb.TagNumber(${oneofTags[0]})
  void clearText() => \$_clearField(${oneofTags[0]});
''');
  idx++;
  b.writeln('''  @\$pb.TagNumber(${oneofTags[1]})
  \$core.int get number => \$_getIZ($idx);
  @\$pb.TagNumber(${oneofTags[1]})
  set number(\$core.int value) => \$_setSignedInt32($idx, value);
  @\$pb.TagNumber(${oneofTags[1]})
  \$core.bool hasNumber() => \$_has($idx);
  @\$pb.TagNumber(${oneofTags[1]})
  void clearNumber() => \$_clearField(${oneofTags[1]});
}
''');
  return 'enum ${n}_Choice { text, number, notSet }\n\n${b}';
}

String ctor(F f) => switch (f.kind) {
      'string' => '\$core.String?',
      'int32' => '\$core.int?',
      'int64' => '\$fixnum.Int64?',
      'bool' => '\$core.bool?',
      'double' => '\$core.double?',
      'bytes' => '\$core.List<\$core.int>?',
      'rstring' => '\$core.Iterable<\$core.String>?',
      'map' => '\$core.Iterable<\$core.MapEntry<\$core.String, \$core.String>>?',
      'enum' => 'Status?',
      'message' => '${f.msg}?',
      'rmessage' => '\$core.Iterable<${f.msg}>?',
      _ => throw f.kind,
    };

String info(F f, String n) {
  final nm = "_omitFieldNames ? '' : '${camel(f.name)}'";
  final pn = camel(f.name) != f.name ? ", protoName: '${f.name}'" : '';
  return switch (f.kind) {
    'string' => '..aOS(${f.tag}, $nm$pn)',
    'int32' => '..aI(${f.tag}, $nm$pn)',
    'int64' => '..aInt64(${f.tag}, $nm$pn)',
    'bool' => '..aOB(${f.tag}, $nm$pn)',
    'double' => '..aD(${f.tag}, $nm$pn)',
    'bytes' => '..a<\$core.List<\$core.int>>(${f.tag}, $nm, \$pb.PbFieldType.OY$pn)',
    'rstring' => '..pPS(${f.tag}, $nm$pn)',
    'map' => "..m<\$core.String, \$core.String>(${f.tag}, $nm,\n        entryClassName: '$n.${cap(f.name)}Entry',\n        keyFieldType: \$pb.PbFieldType.OS,\n        valueFieldType: \$pb.PbFieldType.OS,\n        packageName: const \$pb.PackageName('bench.bulk'))",
    'enum' => '..aE<Status>(${f.tag}, $nm, enumValues: Status.values)',
    'message' => '..aOM<${f.msg}>(${f.tag}, $nm, subBuilder: ${f.msg}.create)',
    'rmessage' => '..pPM<${f.msg}>(${f.tag}, $nm, subBuilder: ${f.msg}.create)',
    _ => throw f.kind,
  };
}

String accessors(F f, int i) {
  final c = camel(f.name), C = cap(f.name), t = f.tag;
  String get, set = '', has = '', clear = '';
  has = '  @\$pb.TagNumber($t)\n  \$core.bool has$C() => \$_has($i);\n';
  clear = '  @\$pb.TagNumber($t)\n  void clear$C() => \$_clearField($t);\n';
  switch (f.kind) {
    case 'string':
      get = '\$core.String get $c => \$_getSZ($i);';
      set = 'set $c(\$core.String value) => \$_setString($i, value);';
    case 'int32':
      get = '\$core.int get $c => \$_getIZ($i);';
      set = 'set $c(\$core.int value) => \$_setSignedInt32($i, value);';
    case 'int64':
      get = '\$fixnum.Int64 get $c => \$_getI64($i);';
      set = 'set $c(\$fixnum.Int64 value) => \$_setInt64($i, value);';
    case 'bool':
      get = '\$core.bool get $c => \$_getBF($i);';
      set = 'set $c(\$core.bool value) => \$_setBool($i, value);';
    case 'double':
      get = '\$core.double get $c => \$_getN($i);';
      set = 'set $c(\$core.double value) => \$_setDouble($i, value);';
    case 'bytes':
      get = '\$core.List<\$core.int> get $c => \$_getN($i);';
      set = 'set $c(\$core.List<\$core.int> value) => \$_setBytes($i, value);';
    case 'enum':
      get = 'Status get $c => \$_getN($i);';
      set = 'set $c(Status value) => \$_setField($t, value);';
    case 'message':
      get = '${f.msg} get $c => \$_getN($i);';
      set = 'set $c(${f.msg} value) => \$_setField($t, value);';
      clear += '  @\$pb.TagNumber($t)\n  ${f.msg} ensure$C() => \$_ensure($i);\n';
    case 'rstring':
      return '  @\$pb.TagNumber($t)\n  \$pb.PbList<\$core.String> get $c => \$_getList($i);\n\n';
    case 'rmessage':
      return '  @\$pb.TagNumber($t)\n  \$pb.PbList<${f.msg}> get $c => \$_getList($i);\n\n';
    case 'map':
      return '  @\$pb.TagNumber($t)\n  \$pb.PbMap<\$core.String, \$core.String> get $c => \$_getMap($i);\n\n';
    default:
      throw f.kind;
  }
  return '  @\$pb.TagNumber($t)\n  $get\n  @\$pb.TagNumber($t)\n  $set\n$has$clear\n';
}

void main(List<String> args) {
  String opt(String k) => args[args.indexOf('--$k') + 1];
  final n = int.parse(opt('messages'));
  final app = opt('app-dir'), pb = opt('pb-dir'), imp = opt('pb-import');
  final methods = n ~/ 4;
  Directory(pb).createSync(recursive: true);
  Directory('$app/lib/src/services').createSync(recursive: true);
  Directory('$app/bin').createSync(recursive: true);

  File('$pb/bulk.pbenum.dart').writeAsStringSync('''${hdr}import 'dart:core' as \$core;

import 'package:protobuf/protobuf.dart' as \$pb;

class Status extends \$pb.ProtobufEnum {
  static const Status OK = Status._(0, _omitEnumNames ? '' : 'OK');
  static const Status WARN = Status._(1, _omitEnumNames ? '' : 'WARN');
  static const Status ERROR = Status._(2, _omitEnumNames ? '' : 'ERROR');

  static const \$core.List<Status> values = <Status>[OK, WARN, ERROR];

  static final \$core.List<Status?> _byValue =
      \$pb.ProtobufEnum.\$_initByValueList(values, 2);
  static Status? valueOf(\$core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const Status._(super.value, super.name);
}

const \$core.bool _omitEnumNames =
    \$core.bool.fromEnvironment('protobuf.omit_enum_names');
''');

  final m = StringBuffer(hdr);
  m.write('''import 'dart:core' as \$core;

import 'package:fixnum/fixnum.dart' as \$fixnum;
import 'package:protobuf/protobuf.dart' as \$pb;

import 'bulk.pbenum.dart';

export 'bulk.pbenum.dart';

''');
  for (var i = 0; i < n; i++) {
    m.write(message(i));
  }
  m.write('''const \$core.bool _omitFieldNames =
    \$core.bool.fromEnvironment('protobuf.omit_field_names');
const \$core.bool _omitMessageNames =
    \$core.bool.fromEnvironment('protobuf.omit_message_names');
''');
  File('$pb/bulk.pb.dart').writeAsStringSync(m.toString());

  final g = StringBuffer(hdr);
  g.write('''import 'dart:async' as \$async;
import 'dart:core' as \$core;

import 'package:grpc/service_api.dart' as \$grpc;
import 'package:protobuf/protobuf.dart' as \$pb;

import 'bulk.pb.dart' as \$0;

export 'bulk.pb.dart';

@\$pb.GrpcServiceName('bench.bulk.Bulk')
abstract class BulkServiceBase extends \$grpc.Service {
  \$core.String get \$name => 'bench.bulk.Bulk';

  BulkServiceBase() {
''');
  for (var j = 0; j < methods; j++) {
    final rq = 'Msg${(4 * j) % n}', rs = 'Msg${(4 * j + 1) % n}';
    g.write('''    \$addMethod(\$grpc.ServiceMethod<\$0.$rq, \$0.$rs>(
        'Op$j',
        op${j}_Pre,
        false,
        false,
        (\$core.List<\$core.int> value) => \$0.$rq.fromBuffer(value),
        (\$0.$rs value) => value.writeToBuffer()));
''');
  }
  g.writeln('  }\n');
  for (var j = 0; j < methods; j++) {
    final rq = 'Msg${(4 * j) % n}', rs = 'Msg${(4 * j + 1) % n}';
    g.write('''  \$async.Future<\$0.$rs> op${j}_Pre(
      \$grpc.ServiceCall \$call, \$async.Future<\$0.$rq> \$request) async {
    return op$j(\$call, await \$request);
  }

''');
  }
  for (var j = 0; j < methods; j++) {
    final rq = 'Msg${(4 * j) % n}', rs = 'Msg${(4 * j + 1) % n}';
    g.writeln('  \$async.Future<\$0.$rs> op$j(\$grpc.ServiceCall call, \$0.$rq request);');
  }
  g.writeln('}');
  File('$pb/bulk.pbgrpc.dart').writeAsStringSync(g.toString());

  // The service of the app: each handler answers with the next message, copying a field of the request.
  // Op0: vuln (a field of the request is a command). Op1: vuln (a field of a nested message is a path).
  // Op2: fixed (a message built here; the field that reaches the sink is a constant, the request's goes to another field).
  // Op3: vuln (the other field of the same locally built message reaches the sink).
  final s = StringBuffer('''import 'dart:io';

import 'package:grpc/grpc.dart';

import '$imp';

class BulkService extends BulkServiceBase {
''');
  for (var j = 0; j < methods; j++) {
    final a = (4 * j) % n, b = (4 * j + 1) % n;
    s.writeln('  @override\n  Future<Msg$b> op$j(ServiceCall call, Msg$a request) async {');
    if (j == 0) {
      s.writeln('    await Process.run(request.name, [request.text]); // vuln');
    } else if (j == 1 && a > 0) {
      s.writeln('    await File(request.child.name).readAsString(); // vuln: a field of a nested message');
    } else if (j == 2) {
      s.writeln("    final m = Msg$a()\n      ..name = request.name\n      ..tags.add('const');");
      s.writeln("    await Process.run('echo', [m.tags.first]); // fixed: the list holds a constant, the request is in another field");
    } else if (j == 3) {
      s.writeln('    final m = Msg$a()..text = request.name;');
      s.writeln('    await Process.run(m.text, []); // vuln: the field the request went into');
    }
    s.writeln('    return Msg$b()..name = request.name;\n  }\n');
  }
  s.writeln('}');
  File('$app/lib/src/services/bulk_service.dart').writeAsStringSync(s.toString());
  File('$app/bin/bulk_server.dart').writeAsStringSync('''import 'package:grpc/grpc.dart';
import 'package:grpc_bench/src/services/bulk_service.dart';

Future<void> main() async {
  final server = Server.create(services: [BulkService()]);
  await server.serve(port: 50052);
}
''');
}
