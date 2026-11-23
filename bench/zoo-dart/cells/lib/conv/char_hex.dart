// @cell(F/conv/char-hex)
// @shape: a home-made hex encoder (two table lookups per code unit of s.codeUnits) and decoder (two digits make a code unit)
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-hex/vuln', vuln);
  zooCase('F/conv/char-hex/fixed', fixed);
  zooCase('F/conv/char-hex/decoy', decoy);
}

const digits = '0123456789abcdef';

String encode(String s) {
  final out = <int>[];
  for (final c in s.codeUnits) {
    out.add(digits.codeUnitAt(c >> 4));
    out.add(digits.codeUnitAt(c & 15));
  }
  return String.fromCharCodes(out);
}

int unhex(int c) => c >= 0x61 ? c - 0x61 + 10 : c - 0x30;

String decode(String h) {
  final u = h.codeUnits;
  final out = <int>[];
  for (var i = 0; i + 1 < u.length; i += 2) {
    out.add((unhex(u[i]) << 4) | unhex(u[i + 1]));
  }
  return String.fromCharCodes(out);
}

String f(String s) => decode(encode(s));

void vuln() { // @vuln
  final v = src('conv-char-hex-vuln'); // @source(conv-char-hex-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-hex-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-hex-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-hex-decoy'); // @source(conv-char-hex-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-hex-decoy)
  sinkSql(f(a)); // @sink(conv-char-hex-decoy)
}
