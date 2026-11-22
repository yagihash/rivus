// The skeleton of `GitHub` of package:github: real code (the harness runs it), not read by rivus (the built-in rows of
// package:github stand for it). It records the path of a request and sends nothing.
import 'package:zoo_mark/zoo_mark.dart';

class GitHub {
  GitHub();

  Future<void> request(String method, String path) async {
    recordAny(path);
  }
}
