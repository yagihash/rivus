// The skeleton of `MultipartFile` of package:dio: `fromFile` and `fromFileSync` take the path of the file that is read and
// sent. It is real code, not a declaration: the harness runs it and it records the path as what reached the sink; rivus does
// not read it (the built-in rows of package:dio stand for the body of a dependency's method).
import 'package:zoo_mark/zoo_mark.dart';

class MultipartFile {
  MultipartFile._();

  static Future<MultipartFile> fromFile(String filePath, {String? filename}) async {
    recordAny(filePath);
    return MultipartFile._();
  }

  static MultipartFile fromFileSync(String filePath, {String? filename}) {
    recordAny(filePath);
    return MultipartFile._();
  }
}
