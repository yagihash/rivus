// The recording implementation of package:zoo_mark (docs/DESIGN-zoo.md, 3.2 and 3.4). The analysis reads
// bench/zoo-dart/cells/packages/zoo_mark, which has the same declarations with empty bodies; this one is
// put in its place by the harness (dependency_overrides) and is no part of what rivus analyses.
//
// The marker and the judgement of "reached" are in bench/zoo/spec.json; the constants below are its copy,
// and test/spec_test.dart holds the two to be the same.
import 'dart:async';
import 'dart:convert';
import 'dart:io';

/// The class of a sink.
const classAny = 'Any';
const classCmd = 'Cmd';
const classSql = 'SQL';
const classHtml = 'HTML';
const classPath = 'Path';

const order = [classCmd, classSql, classHtml, classPath];

class ClassSpec {
  const ClassSpec(this.segment, this.danger, this.set);
  final String segment;
  final List<String> danger;
  final String set;
}

const specs = <String, ClassSpec>{
  classCmd: ClassSpec('.SH', [';|\$', '|\$;'], ';|\$\n'),
  classSql: ClassSpec('.SQL', ['\'"', '\'`'], '\'"\\`'),
  classHtml: ClassSpec('.HTML', ['<>&', '<&>'], '<>&'),
  classPath: ClassSpec('.PATH', ['//../..', '/../../'], './'),
};

/// The variant that this process runs (0 or 1): ZOO_VARIANT.
final int variant = Platform.environment['ZOO_VARIANT'] == '1' ? 1 : 0;

/// The danger of the class in the variant that runs.
String danger(String c) => specs[c]!.danger[variant];

/// One call of a sink.
class Hit {
  Hit(this.caseId, this.file, this.line, this.cls, this.seen, this.reached, this.srcs, this.recv, this.masked, this.ambiguous);
  final String caseId;
  final String file;
  final int line;
  final String cls;
  final List<String> seen;
  final List<String> reached;
  final List<String> srcs;
  final String recv;
  final String masked;
  final bool ambiguous;

  Map<String, Object?> toJson() => {
        'case': caseId,
        'file': file,
        'line': line,
        'class': cls,
        'seen': seen,
        'reached': reached,
        'srcs': srcs,
        'recv': recv,
        'masked': masked,
        if (ambiguous) 'ambiguous': true,
      };
}

class _Case {
  _Case(this.id, this.fn);
  final String id;
  final FutureOr<void> Function() fn;
}

String _current = '';
final _caseSrcs = <String, List<String>>{};
final _allTags = <String>{};
final _hits = <Hit>[];
final _cases = <_Case>[];

/// Registers a case.
void zooCase(String id, FutureOr<void> Function() fn) => _cases.add(_Case(id, fn));

/// The ids of the registered cases, in the order of the registration.
List<String> caseIds() => [for (final c in _cases) c.id];

/// Runs the case of the id (the harness makes one test of each).
Future<void> runCase(String id) async {
  final c = _cases.firstWhere((c) => c.id == id);
  _current = c.id;
  await c.fn();
}

/// The hits so far.
List<Hit> hits() => List.of(_hits);

/// The marker of the tag.
String src(String tag) => _marker(tag);

// (the class In makes its text with this too, and not with src: the analysis must not find a second source
// in the recording implementation, which is what TestZooMarkIndependentDart holds)
String _marker(String tag) {
  _allTags.add(tag);
  final l = _caseSrcs.putIfAbsent(_current, () => []);
  if (!l.contains(tag)) l.add(tag);
  final b = StringBuffer('Z$tag');
  for (final c in order) {
    b.write(specs[c]!.segment + specs[c]!.danger[variant]);
  }
  return b.toString();
}

/// What the user gives to the framework in a case: the marker, which is not a source of the rules.
String typed(String tag) => _marker(tag);

/// The source that is a parameter type.
class In {
  In(String tag) : value = _marker(tag);
  final String value;
}

void sinkCmd(String s) => _record(classCmd, '', [s]);
void sinkCmdBytes(List<int> b) => _record(classCmd, '', [utf8.decode(b, allowMalformed: true)]);
void sinkSql(String s) => _record(classSql, '', [s]);
void sinkHtml(String s) => _record(classHtml, '', [s]);
void sinkPath(String base, String s) => _record(classPath, base, [s]);

/// Records the text of the value (the `toString` of the case's type; a list, a map, a record print their
/// elements).
void sinkAny(Object? v) => _record(classAny, '', [v.toString()]);

String _clean(String c, String s) {
  final set = specs[c]!.set;
  final b = StringBuffer();
  for (final r in s.runes) {
    final ch = String.fromCharCode(r);
    b.write(set.contains(ch) ? '_' : ch);
  }
  return b.toString();
}

String cleanCmd(String s) => _clean(classCmd, s);
String cleanSql(String s) => _clean(classSql, s);
String cleanHtml(String s) => _clean(classHtml, s);
String cleanPath(String s) => _clean(classPath, s);

final _frame = RegExp(r'package:zoo_cells/([^\s:)]+\.dart):(\d+):\d+');

/// The place of the call of the sink: the first frame of the cells (not the recording, not the core
/// library that calls a callback).
(String, int) _where() {
  for (final line in StackTrace.current.toString().split('\n')) {
    final m = _frame.firstMatch(line);
    if (m != null) return ('lib/${m.group(1)}', int.parse(m.group(2)!));
  }
  return ('?', 0);
}

void _record(String cls, String base, List<String> strs) {
  final (file, line) = _where();
  final text = strs.join('\x00');
  final lower = text.toLowerCase();
  final seen = [
    for (final tag in _allTags)
      if (lower.contains('z${tag.toLowerCase()}.')) tag
  ]..sort();
  var reached = false;
  for (final s in strs) {
    if (cls == classAny) {
      for (final c in order) {
        reached = reached || judge(c, '/zoo/base', s);
      }
    } else {
      reached = reached || judge(cls, base, s);
    }
  }
  // for the carry: the text without the dangers of the other classes (they differ between the variants
  // and are no part of what this sink is about); an Any sink has no carry
  var masked = text;
  for (final c in order) {
    if (c != cls) {
      masked = masked.replaceAll(specs[c]!.danger[0], '#').replaceAll(specs[c]!.danger[1], '#');
    }
  }
  if (cls == classAny) masked = '';
  final srcs = List<String>.of(_caseSrcs[_current] ?? const []);
  var tagsReached = <String>[];
  var ambiguous = false;
  if (reached) {
    if (seen.isNotEmpty) {
      tagsReached = seen;
    } else if (srcs.length == 1) {
      tagsReached = List.of(srcs);
    } else {
      ambiguous = true;
    }
  }
  _hits.add(Hit(_current, file, line, cls, seen, tagsReached, srcs, text, masked, ambiguous));
}

/// Says whether the danger of the class, in the variant that runs, has reached a sink of the class in the
/// text (the judgement of bench/zoo/spec.json).
bool judge(String c, String base, String s) {
  final d = danger(c);
  switch (c) {
    case classCmd:
      return _judgeCmd(s, d);
    case classSql:
      var i = 0;
      while (true) {
        final j = s.indexOf(d, i);
        if (j < 0) return false;
        final prev = j > 0 ? s.codeUnitAt(j - 1) : 0;
        final next = j + 1 < s.length ? s.codeUnitAt(j + 1) : 0;
        const bs = 0x5c, sq = 0x27;
        if (prev != bs && prev != sq && next != sq) return true;
        i = j + 1;
      }
    case classHtml:
      return s.contains(d);
    case classPath:
      if (!s.contains('..')) return false;
      final b = _clean_(base);
      final j = _clean_('$base/$s');
      return !(j == b || j.startsWith('$b/'));
  }
  return false;
}

/// The lexical clean of an absolute path (Go's filepath.Clean).
String _clean_(String p) {
  final out = <String>[];
  for (final part in p.split('/')) {
    if (part.isEmpty || part == '.') continue;
    if (part == '..') {
      if (out.isNotEmpty) out.removeLast();
      continue;
    }
    out.add(part);
  }
  return '/${out.join('/')}';
}

bool _judgeCmd(String s, String d) {
  var quote = ''; // '', "'" or '"'
  for (var i = 0; i < s.length; i++) {
    final c = s[i];
    if (quote.isEmpty && s.startsWith(d, i)) {
      return true;
    } else if (c == '\\' && quote != "'") {
      i++;
    } else if (quote.isEmpty && (c == "'" || c == '"')) {
      quote = c;
    } else if (quote.isNotEmpty && c == quote) {
      quote = '';
    }
  }
  return false;
}
