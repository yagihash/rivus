// @cell(F/dyn/sanitizer-own-impl)
// @shape: the project implements an interface whose method the rules call a sanitizer, and the implementation reaches a sink itself
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sanitizer-own-impl/vuln', vuln);
  zooCase('F/dyn/sanitizer-own-impl/fixed', fixed);
  zooCase('F/dyn/sanitizer-own-impl/decoy', decoy);
}

/// Cleaner is declared a sanitizer by rules.yaml: what clean returns is clean.
abstract class Cleaner {
  String clean(String s);
}

class ShellCleaner implements Cleaner {
  @override
  String clean(String s) {
    sinkCmd(s); // @sink(dyn-sanitizer-own-impl-vuln) @noisy(2) @ref(DYN-IMPLS)
    return cleanCmd(s);
  }
}

void vuln() { // @vuln
  final Cleaner c = ShellCleaner();
  c.clean(src('dyn-sanitizer-own-impl-vuln')); // @source(dyn-sanitizer-own-impl-vuln)
}

class FixedCleaner implements Cleaner {
  @override
  String clean(String s) {
    final clean = cleanCmd(s);
    sinkCmd(clean); // @sink(none)
    return clean;
  }
}

void fixed() { // @fixed
  final Cleaner c = FixedCleaner();
  c.clean(src('dyn-sanitizer-own-impl-fixed'));
}

class QuietCleaner implements Cleaner {
  @override
  String clean(String s) {
    sinkCmd(s); // @sink(none) @fp @nosink(dyn-sanitizer-own-impl-decoy) @fp @ref(XTALK)
    return cleanCmd(s);
  }
}

class LoudCleaner implements Cleaner {
  @override
  String clean(String s) {
    sinkSql(s); // @sink(dyn-sanitizer-own-impl-decoy) @noisy(2) @ref(DYN-IMPLS)
    return cleanSql(s);
  }
}

void decoy() { // @decoy
  final Cleaner quiet = QuietCleaner();
  final Cleaner loud = LoudCleaner();
  quiet.clean('constant');
  loud.clean(src('dyn-sanitizer-own-impl-decoy')); // @source(dyn-sanitizer-own-impl-decoy)
}
