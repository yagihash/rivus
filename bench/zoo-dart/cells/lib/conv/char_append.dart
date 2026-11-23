// @cell(F/conv/char-append)
// @shape: the code units of the text are read one by one (codeUnitAt(i)) and added to a list of ints that String.fromCharCodes turns back into a string
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-append/vuln', vuln);
  zooCase('F/conv/char-append/fixed', fixed);
  zooCase('F/conv/char-append/decoy', decoy);
}

String f(String s) {
  final out = <int>[];
  for (var i = 0; i < s.length; i++) {
    out.add(s.codeUnitAt(i));
  }
  return String.fromCharCodes(out);
}

void vuln() { // @vuln
  final v = src('conv-char-append-vuln'); // @source(conv-char-append-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-append-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-append-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-append-decoy'); // @source(conv-char-append-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-append-decoy)
  sinkSql(f(a)); // @sink(conv-char-append-decoy)
}
