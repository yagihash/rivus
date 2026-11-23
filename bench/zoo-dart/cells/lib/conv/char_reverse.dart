// @cell(F/conv/char-reverse)
// @shape: the code units of the text (s.codeUnits) are reversed, and reversed once more
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-reverse/vuln', vuln);
  zooCase('F/conv/char-reverse/fixed', fixed);
  zooCase('F/conv/char-reverse/decoy', decoy);
}

String reverse(String s) => String.fromCharCodes(s.codeUnits.reversed.toList());

String f(String s) => reverse(reverse(s));

void vuln() { // @vuln
  final v = src('conv-char-reverse-vuln'); // @source(conv-char-reverse-vuln)
  sinkCmd(f(v)); // @sink(conv-char-reverse-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-reverse-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-reverse-decoy'); // @source(conv-char-reverse-decoy)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-reverse-decoy)
  sinkSql(f(a)); // @sink(conv-char-reverse-decoy)
}
