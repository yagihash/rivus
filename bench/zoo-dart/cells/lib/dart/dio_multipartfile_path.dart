// @cell(F/dart/dio-multipartfile-path)
// @shape: a value goes into the path that MultipartFile.fromFile and fromFileSync of package:dio (a skeleton of it) read the file from: the path traversal sinks of the built-in rows
import 'package:dio/dio.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/dio-multipartfile-path/vuln', vuln);
  zooCase('F/dart/dio-multipartfile-path/fixed', fixed);
  zooCase('F/dart/dio-multipartfile-path/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final v = src('dart-dio-multipartfile-path-vuln'); // @source(dart-dio-multipartfile-path-vuln)
  await MultipartFile.fromFile(v); // @sink(dart-dio-multipartfile-path-vuln)
  MultipartFile.fromFileSync(v); // @sink(dart-dio-multipartfile-path-vuln)
}

Future<void> fixed() async { // @fixed
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-dio-multipartfile-path-fixed')))));
  await MultipartFile.fromFile(v); // @sink(none)
  MultipartFile.fromFileSync(v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final v = src('dart-dio-multipartfile-path-decoy'); // @source(dart-dio-multipartfile-path-decoy)
  await MultipartFile.fromFile('out.bin', filename: v); // @sink(none) @nosink(dart-dio-multipartfile-path-decoy)
  MultipartFile.fromFileSync('out.bin', filename: v); // @sink(none) @nosink(dart-dio-multipartfile-path-decoy)
  sinkAny(v); // @sink(dart-dio-multipartfile-path-decoy)
}
