// @cell(F/dart/dio-geturi)
// @shape: a value goes into the Uri of Dio.getUri of package:dio (a skeleton of it): the SSRF sink of the built-in rows
import 'package:dio/dio.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/dio-geturi/vuln', vuln);
  zooCase('F/dart/dio-geturi/fixed', fixed);
  zooCase('F/dart/dio-geturi/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final dio = Dio();
  final v = src('dart-dio-geturi-vuln'); // @source(dart-dio-geturi-vuln)
  await dio.getUri(Uri.parse('http://example.invalid/?q=$v')); // @sink(dart-dio-geturi-vuln)
}

Future<void> fixed() async { // @fixed
  final dio = Dio();
  final v = cleanUrl(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-dio-geturi-fixed'))))));
  await dio.getUri(Uri.parse('http://example.invalid/?q=$v')); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final dio = Dio();
  final v = src('dart-dio-geturi-decoy'); // @source(dart-dio-geturi-decoy)
  await dio.getUri(Uri.parse('http://example.invalid/'), data: v); // @sink(none) @nosink(dart-dio-geturi-decoy)
  sinkAny(v); // @sink(dart-dio-geturi-decoy)
}
