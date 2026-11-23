// @cell(F/dart/flutter-field-read)
// @na(go: no such form; the framework's field that holds what the user typed)
// @shape: the source is the read of a field of the framework (the text of a TextEditingController); the case types with typed(), which is not a source of the rules
import 'package:zoo_flutter/zoo_flutter.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/flutter-field-read/vuln', vuln);
  zooCase('F/dart/flutter-field-read/fixed', fixed);
  zooCase('F/dart/flutter-field-read/decoy', decoy);
}

void vuln() { // @vuln
  final c = TextEditingController();
  c.text = typed('dart-flutter-field-read-vuln');
  final v = c.text; // @source(dart-flutter-field-read-vuln)
  sinkCmd(v); // @sink(dart-flutter-field-read-vuln)
}

void fixed() { // @fixed
  final c = TextEditingController();
  c.text = typed('dart-flutter-field-read-fixed');
  final v = cleanCmd(c.text);
  sinkCmd(v); // @sink(none)
}

void decoy() { // @decoy
  final c = TextEditingController();
  c.text = typed('dart-flutter-field-read-decoy');
  final v = c.text; // @source(dart-flutter-field-read-decoy)
  sinkCmd('constant'); // @sink(none) @nosink(dart-flutter-field-read-decoy)
  sinkSql(v); // @sink(dart-flutter-field-read-decoy)
}
