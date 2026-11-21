// @cell(F/dyn/fprint-writer-own)
// @shape: a value is written with write and writeln to a StringSink of the project, and read back from the field that its write stores it in
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/fprint-writer-own/vuln', vuln);
  zooCase('F/dyn/fprint-writer-own/fixed', fixed);
  zooCase('F/dyn/fprint-writer-own/decoy', decoy);
}

class SliceSink implements StringSink {
  String data = '';

  @override
  void write(Object? object) {
    data = data + object.toString();
  }

  @override
  void writeAll(Iterable<dynamic> objects, [String separator = '']) {
    data = data + objects.join(separator);
  }

  @override
  void writeCharCode(int charCode) {
    data = data + String.fromCharCode(charCode);
  }

  @override
  void writeln([Object? object = '']) {
    data = data + object.toString() + '\n';
  }
}

void vuln() { // @vuln
  final w = SliceSink();
  w.write(src('dyn-fprint-writer-own-vuln')); // @source(dyn-fprint-writer-own-vuln)
  w.writeln('');
  w.writeAll(['']);
  w.writeCharCode(10);
  sinkCmd(w.data); // @sink(dyn-fprint-writer-own-vuln)
}

void fixed() { // @fixed
  final w = SliceSink();
  w.write(src('dyn-fprint-writer-own-fixed'));
  w.writeln('');
  w.writeAll(['']);
  w.writeCharCode(10);
  sinkCmd(cleanCmd(w.data)); // @sink(none)
}

class NamedSink implements StringSink {
  String name = 'constant';
  String data = '';

  @override
  void write(Object? object) {
    data = data + object.toString();
  }

  @override
  void writeAll(Iterable<dynamic> objects, [String separator = '']) {
    data = data + objects.join(separator);
  }

  @override
  void writeCharCode(int charCode) {
    data = data + String.fromCharCode(charCode);
  }

  @override
  void writeln([Object? object = '']) {
    data = data + object.toString() + '\n';
  }
}

void decoy() { // @decoy
  final w = NamedSink();
  w.write(src('dyn-fprint-writer-own-decoy')); // @source(dyn-fprint-writer-own-decoy)
  w.writeln('');
  w.writeAll(['']);
  w.writeCharCode(10);
  sinkCmd(w.name); // @sink(none) @nosink(dyn-fprint-writer-own-decoy)
  sinkSql(w.data); // @sink(dyn-fprint-writer-own-decoy)
}
