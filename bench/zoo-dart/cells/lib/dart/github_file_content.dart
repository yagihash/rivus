// @cell(F/dart/github-file-content)
// @shape: a value that a user of GitHub wrote is read with GitHubFile.content of package:github (a skeleton of it): no row makes it a source (a known miss)
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-file-content/vuln', vuln);
  zooCase('F/dart/github-file-content/fixed', fixed);
  zooCase('F/dart/github-file-content/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final file = GitHubFile(content: typed('dart-github-file-content-vuln'));
  sinkCmd(file.content!); // @source(dart-github-file-content-vuln) @miss(silent) @ref(model-github-field-source-in-dependency) @sink(dart-github-file-content-vuln)
}

Future<void> fixed() async { // @fixed
  final file = GitHubFile(content: typed('dart-github-file-content-fixed'));
  sinkCmd(cleanCmd(file.content!)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final file = GitHubFile(content: typed('dart-github-file-content-decoy'));
  final v = file.content!; // @source(dart-github-file-content-decoy) @miss(silent) @ref(model-github-field-source-in-dependency)
  sinkCmd('constant'); // @sink(none) @nosink(dart-github-file-content-decoy)
  sinkSql(v); // @sink(dart-github-file-content-decoy)
}
