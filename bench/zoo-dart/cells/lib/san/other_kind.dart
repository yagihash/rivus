// @cell(F/san/other-kind)
// @shape: a sanitizer makes the value safe for the sinks of its own kind only (an HTML clean is no clean for a command or a query): the value goes on to the other kinds of sinks
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/san/other-kind/vuln', vuln);
  zooCase('F/san/other-kind/vuln2', vuln2);
  zooCase('F/san/other-kind/fixed', fixed);
  zooCase('F/san/other-kind/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('san-other-kind-vuln'); // @source(san-other-kind-vuln)
  sinkCmd(cleanHtml(v)); // @sink(san-other-kind-vuln)
}

void vuln2() { // @vuln
  final v = src('san-other-kind-vuln2'); // @source(san-other-kind-vuln2)
  sinkHtml(cleanCmd(v)); // @sink(san-other-kind-vuln2)
}

void fixed() { // @fixed
  final v = src('san-other-kind-fixed');
  sinkHtml(cleanHtml(v)); // @sink(none)
  sinkCmd(cleanCmd(v)); // @sink(none)
}

void decoy() { // @decoy
  final v = src('san-other-kind-decoy'); // @source(san-other-kind-decoy)
  final h = cleanHtml(v);
  sinkHtml(h); // @sink(none) @nosink(san-other-kind-decoy)
  sinkSql(h); // @sink(san-other-kind-decoy)
}
