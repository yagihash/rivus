// @cell(F/dart/async-await)
// @shape: a value goes through an async function and an await to a sink (the decoy awaits the constant and sinks the tainted value elsewhere)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/async-await/vuln', vuln);
  zooCase('F/dart/async-await/fixed', fixed);
  zooCase('F/dart/async-await/decoy', decoy);
}

Future<String> load(String s) async {
  await Future<void>.delayed(Duration.zero);
  return s;
}

Future<void> vuln() async { // @vuln
  final v = await load(src('dart-async-await-vuln')); // @source(dart-async-await-vuln)
  sinkCmd(v); // @sink(dart-async-await-vuln)
}

Future<String> loadFixed(String s) async {
  await Future<void>.delayed(Duration.zero);
  return s;
}

Future<void> fixed() async { // @fixed
  final v = await loadFixed(cleanCmd(src('dart-async-await-fixed')));
  sinkCmd(v); // @sink(none)
}

Future<String> loadDecoy(String s) async {
  await Future<void>.delayed(Duration.zero);
  return s;
}

Future<void> decoy() async { // @decoy
  final t = src('dart-async-await-decoy'); // @source(dart-async-await-decoy)
  final c = await loadDecoy('constant');
  sinkCmd(c); // @sink(none) @nosink(dart-async-await-decoy)
  sinkSql(await loadDecoy(t)); // @sink(dart-async-await-decoy)
}
