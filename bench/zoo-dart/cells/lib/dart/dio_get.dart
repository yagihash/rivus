// @cell(F/dart/dio-get)
// @shape: a value goes into the path (the URL) of Dio.get of package:dio (a skeleton of it): the SSRF sink of the built-in rows
import 'package:dio/dio.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/dio-get/vuln', vuln);
  zooCase('F/dart/dio-get/fixed', fixed);
  zooCase('F/dart/dio-get/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final dio = Dio();
  final v = src('dart-dio-get-vuln'); // @source(dart-dio-get-vuln)
  await dio.get('http://example.invalid/?q=$v'); // @sink(dart-dio-get-vuln)
}

Future<void> fixed() async { // @fixed
  final dio = Dio();
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-dio-get-fixed')))));
  await dio.get('http://example.invalid/?q=$v'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final dio = Dio();
  final v = src('dart-dio-get-decoy'); // @source(dart-dio-get-decoy)
  await dio.get('http://example.invalid/', data: v); // @sink(none) @nosink(dart-dio-get-decoy)
  sinkAny(v); // @sink(dart-dio-get-decoy)
}
