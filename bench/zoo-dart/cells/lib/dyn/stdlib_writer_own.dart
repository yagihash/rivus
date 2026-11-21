// @cell(F/dyn/stdlib-writer-own)
// @shape: the project implements Sink<String> (the interface of the SDK), and a value added through the interface reaches a sink in its add
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/stdlib-writer-own/vuln', vuln);
  zooCase('F/dyn/stdlib-writer-own/fixed', fixed);
  zooCase('F/dyn/stdlib-writer-own/decoy', decoy);
}

class ShellSink implements Sink<String> {
  @override
  void add(String data) {
    sinkCmd(data); // @sink(dyn-stdlib-writer-own-vuln) @noisy(2) @ref(DYN-IMPLS)
  }

  @override
  void close() {}
}

void vuln() { // @vuln
  final Sink<String> out = ShellSink();
  out.add(src('dyn-stdlib-writer-own-vuln')); // @source(dyn-stdlib-writer-own-vuln)
  out.close();
}

class FixedSink implements Sink<String> {
  @override
  void add(String data) {
    sinkCmd(cleanCmd(data)); // @sink(none)
  }

  @override
  void close() {}
}

void fixed() { // @fixed
  final Sink<String> out = FixedSink();
  out.add(src('dyn-stdlib-writer-own-fixed'));
  out.close();
}

class QuietSink implements Sink<String> {
  @override
  void add(String data) {
    sinkCmd(data); // @sink(none) @fp @nosink(dyn-stdlib-writer-own-decoy) @fp @ref(XTALK)
  }

  @override
  void close() {}
}

class LoudSink implements Sink<String> {
  @override
  void add(String data) {
    sinkSql(data); // @sink(dyn-stdlib-writer-own-decoy) @noisy(2) @ref(DYN-IMPLS)
  }

  @override
  void close() {}
}

void decoy() { // @decoy
  final Sink<String> quiet = QuietSink();
  final Sink<String> loud = LoudSink();
  quiet.add('constant');
  loud.add(src('dyn-stdlib-writer-own-decoy')); // @source(dyn-stdlib-writer-own-decoy)
  quiet.close();
  loud.close();
}
