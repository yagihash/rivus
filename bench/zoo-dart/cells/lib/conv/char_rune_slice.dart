// @cell(F/conv/char-rune-slice)
// @shape: the text is converted whole to a list of runes (s.runes.toList()), sliced, and converted back whole with String.fromCharCodes (no element is read)
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-rune-slice/vuln', vuln);
  zooCase('F/conv/char-rune-slice/fixed', fixed);
  zooCase('F/conv/char-rune-slice/decoy', decoy);
}

String f(String s) {
  final rs = s.runes.toList();
  return String.fromCharCodes(rs.sublist(0, 1)) + String.fromCharCodes(rs.sublist(1));
}

void vuln() { // @vuln
  final v = src('conv-char-rune-slice-vuln'); // @source(conv-char-rune-slice-vuln)
  sinkCmd(f(v)); // @sink(conv-char-rune-slice-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-rune-slice-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-rune-slice-decoy'); // @source(conv-char-rune-slice-decoy)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-rune-slice-decoy)
  sinkSql(f(a)); // @sink(conv-char-rune-slice-decoy)
}
