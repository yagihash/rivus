// @cell(F/dart/github-issue-body)
// @elsewhere(go: F/pkg/go-github/sources)
// @shape: a value that a user of GitHub wrote is read with Issue.body of package:github (a skeleton of it): the field source of the built-in rows
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-issue-body/vuln', vuln);
  zooCase('F/dart/github-issue-body/fixed', fixed);
  zooCase('F/dart/github-issue-body/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final file = Issue(body: typed('dart-github-issue-body-vuln'));
  sinkCmd(file.body!); // @source(dart-github-issue-body-vuln) @sink(dart-github-issue-body-vuln)
}

Future<void> fixed() async { // @fixed
  final file = Issue(body: typed('dart-github-issue-body-fixed'));
  sinkCmd(cleanCmd(file.body!)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final file = Issue(body: typed('dart-github-issue-body-decoy'));
  final v = file.body!; // @source(dart-github-issue-body-decoy)
  sinkCmd('constant'); // @sink(none) @nosink(dart-github-issue-body-decoy)
  sinkSql(v); // @sink(dart-github-issue-body-decoy)
}
