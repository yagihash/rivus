// @cell(F/dart/future-then)
// @shape: a value goes through the callback of Future.then to a sink (the decoy's callback gets the other future's result)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/future-then/vuln', vuln);
  zooCase('F/dart/future-then/fixed', fixed);
  zooCase('F/dart/future-then/decoy', decoy);
}

Future<void> vuln() { // @vuln
  return Future.value(src('dart-future-then-vuln')).then((v) { // @source(dart-future-then-vuln)
    sinkCmd(v); // @sink(dart-future-then-vuln)
  });
}

Future<void> fixed() { // @fixed
  return Future.value(cleanCmd(src('dart-future-then-fixed'))).then((v) {
    sinkCmd(v); // @sink(none)
  });
}

Future<void> decoy() async { // @decoy
  final t = src('dart-future-then-decoy'); // @source(dart-future-then-decoy)
  await Future.value('constant').then((v) {
    sinkCmd(v); // @sink(none) @nosink(dart-future-then-decoy)
  });
  await Future.value(t).then((v) {
    sinkSql(v); // @sink(dart-future-then-decoy)
  });
}
