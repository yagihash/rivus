// @cell(F/dart/dio-post)
// @shape: a value goes into the path (the URL) of Dio.post of package:dio (a skeleton of it): the SSRF sink of the built-in rows
import 'package:dio/dio.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/dio-post/vuln', vuln);
  zooCase('F/dart/dio-post/fixed', fixed);
  zooCase('F/dart/dio-post/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final dio = Dio();
  final v = src('dart-dio-post-vuln'); // @source(dart-dio-post-vuln)
  await dio.post('http://example.invalid/?q=$v'); // @sink(dart-dio-post-vuln)
}

Future<void> fixed() async { // @fixed
  final dio = Dio();
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-dio-post-fixed')))));
  await dio.post('http://example.invalid/?q=$v'); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final dio = Dio();
  final v = src('dart-dio-post-decoy'); // @source(dart-dio-post-decoy)
  await dio.post('http://example.invalid/', data: v); // @sink(none) @nosink(dart-dio-post-decoy)
  sinkAny(v); // @sink(dart-dio-post-decoy)
}
