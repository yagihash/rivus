// @cell(F/conv/char-case)
// @shape: every code unit of the text is read with codeUnitAt, converted to lower case by arithmetic and written with StringBuffer.writeCharCode
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-case/vuln', vuln);
  zooCase('F/conv/char-case/fixed', fixed);
  zooCase('F/conv/char-case/decoy', decoy);
}

String f(String s) {
  final b = StringBuffer();
  for (var i = 0; i < s.length; i++) {
    var c = s.codeUnitAt(i);
    if (c >= 0x41 && c <= 0x5a) c += 0x20;
    b.writeCharCode(c);
  }
  return b.toString();
}

void vuln() { // @vuln
  final v = src('conv-char-case-vuln'); // @source(conv-char-case-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-case-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-case-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-case-decoy'); // @source(conv-char-case-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-case-decoy)
  sinkSql(f(a)); // @sink(conv-char-case-decoy)
}
