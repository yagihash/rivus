import 'helpers.dart';
import 'lib.dart';

// Functions as values: tear-offs of methods, static methods, constructors, and
// function-typed fields.

class Cleaner {
  String keep(String s) => s;
  String drop(String s) => 'fixed';
  static String shout(String s) => s.toUpperCase();
}

void method_tearoff_keep() {
  final c = Cleaner();
  final a = source(); // @source(tear-keep)
  sink(transform(a, c.keep)); // @sink(tear-keep)
}

void method_tearoff_drop() {
  final c = Cleaner();
  final a = source(); // @source(tear-drop)
  sink(transform(a, c.drop)); // @sink(none) @fp
  sink(a); // @sink(tear-drop)
}

void static_tearoff() {
  final a = source(); // @source(tear-static)
  sink(transform(a, Cleaner.shout)); // @sink(tear-static)
}

void toplevel_tearoff() {
  final a = source(); // @source(tear-top)
  sink(transform(a, helperKeep)); // @sink(tear-top)
}

String helperKeep(String s) => s;

class Box {
  final String v;
  Box(this.v);
}

void ctor_tearoff() {
  final a = source(); // @source(tear-ctor)
  final boxes = [a].map(Box.new).toList();
  sink(boxes.first.v); // @sink(tear-ctor)
}

// the tear-off is a sink itself
void sink_tearoff() {
  final items = [source()]; // @source(tear-sink)
  items.forEach(sink); // @sink(tear-sink)
}

// a function-typed field of an object
class Handler {
  final String Function(String) on;
  Handler(this.on);
}

void field_function_a() {
  final a = source(); // @source(field-fn-a)
  final h = Handler((x) => x);
  sink(h.on(a)); // @sink(field-fn-a) @nosink(field-fn-b)
}

void field_function_b() {
  final b = source(); // @source(field-fn-b)
  final h = Handler((x) => x);
  sink(h.on(b)); // @sink(field-fn-b) @nosink(field-fn-a)
}

// the callback is kept in the object and called by a method
void runner_a() {
  final a = source(); // @source(runner-a)
  final r = Runner((x) => x);
  sink(r.go(a)); // @sink(runner-a) @nosink(runner-b)
}

void runner_b() {
  final b = source(); // @source(runner-b)
  final r = Runner((x) => x);
  sink(r.go(b)); // @sink(runner-b) @nosink(runner-a)
}

// a table of functions
final Map<String, String Function(String)> table = {
  'keep': (x) => x,
  'drop': (x) => 'fixed',
};

void table_lookup() {
  final a = source(); // @source(table)
  sink(table['keep']!(a)); // @sink(table)
}

// a method that is given `this.method` as the callback
class Pipeline {
  final String prefix;
  Pipeline(this.prefix);
  String stamp(String s) => prefix + s;
  String run(String s) => transform(s, stamp);
}

void bound_method() {
  final a = source(); // @source(bound)
  sink(Pipeline('p').run(a)); // @sink(bound)
}

void bound_method_prefix() {
  final p = source(); // @source(bound-prefix) @miss(silent)
  sink(Pipeline(p).run('const')); // @sink(bound-prefix)
}
