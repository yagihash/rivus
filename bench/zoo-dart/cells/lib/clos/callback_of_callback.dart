// @cell(F/clos/callback-of-callback)
// @shape: a closure takes a function as its parameter and calls it (g(a)): the callback of a callback
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/clos/callback-of-callback/vuln', vuln);
  zooCase('F/clos/callback-of-callback/fixed', fixed);
  zooCase('F/clos/callback-of-callback/decoy', decoy);
}

String viaInner(String s, String Function(String Function(String)) outer) => outer((x) => x + s);

void vuln() { // @vuln
  final a = src('clos-callback-of-callback-vuln'); // @source(clos-callback-of-callback-vuln) @miss(depth_limit) @ref(#81)
  sinkCmd(viaInner('const', (g) => g(a))); // @sink(clos-callback-of-callback-vuln)
}

void fixed() { // @fixed
  final a = cleanCmd(src('clos-callback-of-callback-fixed'));
  sinkCmd(viaInner('const', (g) => g(a))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('clos-callback-of-callback-decoy'); // @source(clos-callback-of-callback-decoy) @miss(depth_limit) @ref(#81)
  sinkCmd(viaInner('const', (g) => g('constant'))); // @sink(none) @nosink(clos-callback-of-callback-decoy)
  sinkSql(viaInner('const', (g) => g(a))); // @sink(clos-callback-of-callback-decoy)
}
