// @cell(F/conv/char-replace)
// @shape: every rune of the text is read from s.runes, mapped one by one (a replacement) and made a string with String.fromCharCodes
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-replace/vuln', vuln);
  zooCase('F/conv/char-replace/fixed', fixed);
  zooCase('F/conv/char-replace/decoy', decoy);
}

String f(String s) {
  return String.fromCharCodes(s.runes.map((r) => r == 0x53 ? 0x73 : r));
}

void vuln() { // @vuln
  final v = src('conv-char-replace-vuln'); // @source(conv-char-replace-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-replace-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-replace-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-replace-decoy'); // @source(conv-char-replace-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-replace-decoy)
  sinkSql(f(a)); // @sink(conv-char-replace-decoy)
}
