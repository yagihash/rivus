// @cell(F/dyn/through-dependency)
// @shape: a value written through an interface goes through a dependency (zoo_dep.wrap, a package of its own whose body is not read) to a writer of the project, and reaches a sink in its write
// (rivus reads the body of zoo_dep, a path package: the flow is found, and the dependency_body_unread of the Go cell does not come.)
import 'package:zoo_dep/zoo_dep.dart' as dep;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/through-dependency/vuln', vuln);
  zooCase('F/dyn/through-dependency/fixed', fixed);
  zooCase('F/dyn/through-dependency/decoy', decoy);
}

class ShellWriter implements dep.Writer {
  @override
  void write(String data) {
    sinkCmd(data); // @sink(dyn-through-dependency-vuln) @noisy(2) @ref(DYN-IMPLS)
  }
}

void vuln() { // @vuln
  final dep.Writer out = dep.wrap(ShellWriter());
  out.write(src('dyn-through-dependency-vuln')); // @source(dyn-through-dependency-vuln)
}

class FixedWriter implements dep.Writer {
  @override
  void write(String data) {
    sinkCmd(cleanCmd(data)); // @sink(none)
  }
}

void fixed() { // @fixed
  final dep.Writer out = dep.wrap(FixedWriter());
  out.write(src('dyn-through-dependency-fixed'));
}

class QuietWriter implements dep.Writer {
  @override
  void write(String data) {
    sinkCmd(data); // @sink(none) @fp @nosink(dyn-through-dependency-decoy) @fp @ref(XTALK)
  }
}

class LoudWriter implements dep.Writer {
  @override
  void write(String data) {
    sinkSql(data); // @sink(dyn-through-dependency-decoy) @noisy(2) @ref(DYN-IMPLS)
  }
}

void decoy() { // @decoy
  final dep.Writer quiet = dep.wrap(QuietWriter());
  final dep.Writer loud = dep.wrap(LoudWriter());
  quiet.write('constant');
  loud.write(src('dyn-through-dependency-decoy')); // @source(dyn-through-dependency-decoy)
}
