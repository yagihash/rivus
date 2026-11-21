// @cell(F/conv/buffer-writestring)
// @shape: the value is written into a StringBuffer (write, as a StringSink), and buffer.toString() is the sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/buffer-writestring/vuln', vuln);
  zooCase('F/conv/buffer-writestring/fixed', fixed);
  zooCase('F/conv/buffer-writestring/decoy', decoy);
}

void writeString(StringSink out, String s) {
  out.write(s);
}

void vuln() { // @vuln
  final b = StringBuffer();
  writeString(b, src('conv-buffer-writestring-vuln')); // @source(conv-buffer-writestring-vuln)
  sinkCmd(b.toString()); // @sink(conv-buffer-writestring-vuln)
}

void fixed() { // @fixed
  final b = StringBuffer();
  writeString(b, cleanCmd(src('conv-buffer-writestring-fixed')));
  sinkCmd(b.toString()); // @sink(none)
}

void decoy() { // @decoy
  final a = StringBuffer();
  final b = StringBuffer();
  writeString(a, src('conv-buffer-writestring-decoy')); // @source(conv-buffer-writestring-decoy)
  writeString(b, 'constant');
  sinkCmd(b.toString()); // @sink(none) @nosink(conv-buffer-writestring-decoy)
  sinkSql(a.toString()); // @sink(conv-buffer-writestring-decoy)
}
