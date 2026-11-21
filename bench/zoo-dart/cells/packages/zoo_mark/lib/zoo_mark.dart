// What the cases of the zoo call: the sources, the sinks and the sanitizers. This is the file that the
// analysis reads: the declarations only, with empty bodies, so that no state of the recording is seen by
// the analysis (every case would write to one global). rivus gets the meaning of each function from
// rules.yaml. The recording is in bench/zoo-dart/exec/zoo_mark (the harness puts it in place of this
// package with dependency_overrides). The two have the same declarations, and the test of the zoo holds
// them to it.
import 'dart:async';

/// The source that is a parameter type (rules.yaml: param_type). The handler under test takes an [In]; a
/// case makes one with `In('tag')`.
class In {
  In(String tag) : value = tag;
  final String value;
}

/// The marker of the tag (a source).
String src(String tag) => '';

/// The sinks of each class. They record what they got and execute nothing.
void sinkCmd(String s) {}
void sinkCmdBytes(List<int> b) {}
void sinkSql(String s) {}
void sinkHtml(String s) {}
void sinkPath(String base, String s) {}

/// A sink for structs and containers only: the text of the value (its `toString`) is what is recorded.
void sinkAny(Object? v) {}

/// The sanitizers: each replaces the dangerous characters of its class by '_'.
String cleanCmd(String s) => '';
String cleanSql(String s) => '';
String cleanHtml(String s) => '';
String cleanPath(String s) => '';

/// Registers a case of the zoo (the harness runs it).
void zooCase(String id, FutureOr<void> Function() fn) {}
