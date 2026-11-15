// The neutral IR as the Go side reads it (internal/ir: JSON Lines,
// "rivus.ir/0"). Only what the extractor writes is here.
import 'dart:convert';

const schemaVersion = 'rivus.ir/0';

/// A position: a range in a file. The offset counts UTF-16 code units, the
/// way Dart counts; the core only compares offsets of the same file.
class Pos {
  const Pos(this.file, this.offset, this.line, this.col);
  static const none = Pos('', 0, 0, 0);
  final String file;
  final int offset;
  final int line;
  final int col;

  bool get isNone => file.isEmpty;

  Map<String, Object?> toJson() => isNone ? {} : {'file': file, 'offset': offset, 'line': line, 'col': col};
}

class Local {
  Local(this.name, this.type, this.cls);
  final String name;
  final String type;
  final String cls;

  Map<String, Object?> toJson() => {
        if (name.isNotEmpty) 'name': name,
        if (type.isNotEmpty) 'type': type,
        if (cls.isNotEmpty) 'class': cls,
      };
}

class Param {
  Param(this.name, this.type, this.cls, this.display, {this.captured = false});
  final String name;
  final String type;
  final String cls;
  final String display;
  final bool captured;

  Map<String, Object?> toJson() => {
        if (name.isNotEmpty) 'name': name,
        if (type.isNotEmpty) 'type': type,
        if (cls.isNotEmpty) 'class': cls,
        if (display.isNotEmpty) 'display': display,
        if (captured) 'captured': true,
      };
}

/// An instruction, as the map the stream has. Keys left out are the defaults.
typedef Instr = Map<String, Object?>;

class Edge {
  Edge(this.to, this.kind);
  final int to;
  final String kind;

  Map<String, Object?> toJson() => {'to': to, 'kind': kind};
}

class Blk {
  Blk(this.id);
  final int id;
  final List<Instr> instrs = [];
  Map<String, Object?>? term;
  final List<Edge> succs = [];

  Map<String, Object?> toJson() => {
        'id': id,
        if (instrs.isNotEmpty) 'instrs': instrs,
        'term': term ?? {'kind': 'unreachable'},
        if (succs.isNotEmpty) 'succs': succs.map((e) => e.toJson()).toList(),
      };
}

class FuncOut {
  FuncOut(this.id);
  final String id;
  String symbol = '';
  bool recv = false;
  bool exported = false;
  Pos pos = Pos.none;
  final List<Param> params = [];
  final List<String> results = [];
  final List<Local> locals = [];
  final List<Blk> blocks = [];

  Map<String, Object?> toJson() => {
        'id': id,
        if (params.isNotEmpty) 'params': params.map((p) => p.toJson()).toList(),
        if (results.isNotEmpty) 'results': results,
        if (locals.isNotEmpty) 'locals': locals.map((l) => l.toJson()).toList(),
        'blocks': blocks.map((b) => b.toJson()).toList(),
        'roots': [0],
        if (!pos.isNone) 'pos': pos.toJson(),
        if (exported) 'exported': true,
        if (symbol.isNotEmpty) 'symbol': symbol,
        if (recv) 'recv': true,
      };
}

/// Writes the stream.
class StreamOut {
  StreamOut(this.sink) {
    _line({'schema_version': schemaVersion});
  }
  final StringSink sink;

  void _line(Object? o) => sink.writeln(jsonEncode(o));

  void func(FuncOut f) => _line({'kind': 'func', 'func': f.toJson()});
  void global(String name, String type, Pos pos) =>
      _line({'kind': 'global', 'global': {'name': name, if (type.isNotEmpty) 'type': type, if (!pos.isNone) 'pos': pos.toJson()}});
  void implementations(String method, List<String> impls) =>
      _line({'kind': 'implementations', 'implementations': {'method': method, 'impls': impls}});
  void text(String kind, String file, String text, List<int> offsets) =>
      _line({'kind': 'text', 'text': {'kind': kind, 'file': file, 'text': text, 'offsets': offsets}});
  void callText(String file, int offset, String call, List<String> args) => _line({
        'kind': 'call_text',
        'call_text': {'file': file, 'offset': offset, if (call.isNotEmpty) 'call': call, if (args.isNotEmpty) 'args': args}
      });
  void funcText(String id, String body, String source) => _line({
        'kind': 'func_text',
        'func_text': {'func': id, if (body.isNotEmpty) 'body': body, if (source.isNotEmpty) 'source': source}
      });
  void loadError(String package, String message) =>
      _line({'kind': 'load_error', 'load_error': {'package': package, 'message': message}});
}
