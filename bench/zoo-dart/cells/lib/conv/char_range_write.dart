// @cell(F/conv/char-range-write)
// @shape: every rune of the text is read by a for-in loop over s.runes and written with StringBuffer.writeCharCode
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-range-write/vuln', vuln);
  zooCase('F/conv/char-range-write/fixed', fixed);
  zooCase('F/conv/char-range-write/decoy', decoy);
}

String f(String s) {
  final b = StringBuffer();
  for (final r in s.runes) {
    b.writeCharCode(r);
  }
  return b.toString();
}

void vuln() { // @vuln
  final v = src('conv-char-range-write-vuln'); // @source(conv-char-range-write-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-range-write-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-range-write-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-range-write-decoy'); // @source(conv-char-range-write-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-range-write-decoy)
  sinkSql(f(a)); // @sink(conv-char-range-write-decoy)
}
