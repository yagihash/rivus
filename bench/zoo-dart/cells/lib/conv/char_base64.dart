// @cell(F/conv/char-base64)
// @shape: a home-made base64 encoder (three code units of s.codeUnits make four symbols) and decoder, with padding
// @property: text carried through per-character reads and writes reaches the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/char-base64/vuln', vuln);
  zooCase('F/conv/char-base64/fixed', fixed);
  zooCase('F/conv/char-base64/decoy', decoy);
}

const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';

String encode(String s) {
  final u = s.codeUnits;
  final out = <int>[];
  for (var i = 0; i < u.length; i += 3) {
    var n = 0;
    var k = 0;
    for (var j = 0; j < 3; j++) {
      n <<= 8;
      if (i + j < u.length) {
        n |= u[i + j];
        k++;
      }
    }
    out.add(alphabet.codeUnitAt((n >> 18) & 63));
    out.add(alphabet.codeUnitAt((n >> 12) & 63));
    out.add(k > 1 ? alphabet.codeUnitAt((n >> 6) & 63) : 0x3d);
    out.add(k > 2 ? alphabet.codeUnitAt(n & 63) : 0x3d);
  }
  return String.fromCharCodes(out);
}

String decode(String s) {
  final u = s.codeUnits;
  final out = <int>[];
  for (var i = 0; i + 3 < u.length; i += 4) {
    var n = 0;
    var pad = 0;
    for (var j = 0; j < 4; j++) {
      n <<= 6;
      if (u[i + j] == 0x3d) {
        pad++;
      } else {
        n |= alphabet.indexOf(String.fromCharCode(u[i + j]));
      }
    }
    out.add((n >> 16) & 255);
    if (pad < 2) out.add((n >> 8) & 255);
    if (pad < 1) out.add(n & 255);
  }
  return String.fromCharCodes(out);
}

String f(String s) => decode(encode(s));

void vuln() { // @vuln
  final v = src('conv-char-base64-vuln'); // @source(conv-char-base64-vuln) @miss(silent) @ref(#185)
  sinkCmd(f(v)); // @sink(conv-char-base64-vuln)
}

void fixed() { // @fixed
  sinkCmd(f(cleanCmd(src('conv-char-base64-fixed')))); // @sink(none)
}

void decoy() { // @decoy
  final a = src('conv-char-base64-decoy'); // @source(conv-char-base64-decoy) @miss(silent) @ref(#185)
  sinkCmd(f('constant')); // @sink(none) @nosink(conv-char-base64-decoy)
  sinkSql(f(a)); // @sink(conv-char-base64-decoy)
}
