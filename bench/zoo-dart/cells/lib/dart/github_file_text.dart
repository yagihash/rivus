// @cell(F/dart/github-file-text)
// @shape: a value that a user of GitHub wrote is read with GitHubFile.text of package:github (a skeleton of it): the source of the built-in rows
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-file-text/vuln', vuln);
  zooCase('F/dart/github-file-text/fixed', fixed);
  zooCase('F/dart/github-file-text/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final file = GitHubFile(content: typed('dart-github-file-text-vuln'));
  sinkCmd(file.text); // @source(dart-github-file-text-vuln) @sink(dart-github-file-text-vuln)
}

Future<void> fixed() async { // @fixed
  final file = GitHubFile(content: typed('dart-github-file-text-fixed'));
  sinkCmd(cleanCmd(file.text)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final file = GitHubFile(content: typed('dart-github-file-text-decoy'));
  final v = file.text; // @source(dart-github-file-text-decoy)
  sinkCmd('constant'); // @sink(none) @nosink(dart-github-file-text-decoy)
  sinkSql(v); // @sink(dart-github-file-text-decoy)
}
