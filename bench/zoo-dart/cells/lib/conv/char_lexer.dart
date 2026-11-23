// @cell(F/conv/char-lexer)
// @shape: a lexer reads the text one code unit at a time (codeUnitAt(i)), collects the code units of a token and hands out the tokens
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-lexer/vuln', vuln);
  zooCase('F/conv/char-lexer/fixed', fixed);
  zooCase('F/conv/char-lexer/decoy', decoy);
}

String f(String s) {
  final tokens = <String>[];
  var cur = <int>[];
  for (var i = 0; i < s.length; i++) {
    final c = s.codeUnitAt(i);
    if (c == 0x20) {
      tokens.add(String.fromCharCodes(cur));
      cur = <int>[];
      continue;
    }
    cur.add(c);
  }
  tokens.add(String.fromCharCodes(cur));
  return tokens.join(' ');
}

void vuln() { // @vuln
  final v = src('conv-char-lexer-vuln'); // @source(conv-char-lexer-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-lexer-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-lexer-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-lexer-decoy'); // @source(conv-char-lexer-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('a b')); // @sink(none) @nosink(conv-char-lexer-decoy)
  sinkSql(f(a)); // @sink(conv-char-lexer-decoy)
}
