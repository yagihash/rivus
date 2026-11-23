// @cell(F/conv/char-index)
// @shape: the first character is read with s[0] and the rest is a substring; the other characters are read with s[i] and concatenated
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-index/vuln', vuln);
  zooCase('F/conv/char-index/fixed', fixed);
  zooCase('F/conv/char-index/decoy', decoy);
}

String f(String s) {
  var out = s[0];
  for (var i = 1; i < s.length; i++) {
    out += s[i];
  }
  return out;
}

void vuln() { // @vuln
  final v = src('conv-char-index-vuln'); // @source(conv-char-index-vuln)
  sinkCmd(f(v)); // @sink(conv-char-index-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-index-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-index-decoy'); // @source(conv-char-index-decoy)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-index-decoy)
  sinkSql(f(a)); // @sink(conv-char-index-decoy)
}
