// @cell(F/dart/dio-download-savepath)
// @shape: a value goes into the place that Dio.download of package:dio (a skeleton of it) writes the file to: the path traversal sink of the built-in rows
import 'package:dio/dio.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/dio-download-savepath/vuln', vuln);
  zooCase('F/dart/dio-download-savepath/fixed', fixed);
  zooCase('F/dart/dio-download-savepath/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final dio = Dio();
  final v = src('dart-dio-download-savepath-vuln'); // @source(dart-dio-download-savepath-vuln)
  await dio.download('http://example.invalid/file', v); // @sink(dart-dio-download-savepath-vuln)
}

Future<void> fixed() async { // @fixed
  final dio = Dio();
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-dio-download-savepath-fixed')))));
  await dio.download('http://example.invalid/file', v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final dio = Dio();
  final v = src('dart-dio-download-savepath-decoy'); // @source(dart-dio-download-savepath-decoy)
  await dio.download('http://example.invalid/file', 'out.bin', data: v); // @sink(none) @nosink(dart-dio-download-savepath-decoy)
  sinkAny(v); // @sink(dart-dio-download-savepath-decoy)
}
