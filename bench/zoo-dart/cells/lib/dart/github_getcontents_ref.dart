// @cell(F/dart/github-getcontents-ref)
// @na(go: the model of package:github, a Dart package; the Go side is measured in zoo-go-pkg (go_github), not by a cell of the same name)
// @shape: a value goes into the named argument ref of RepositoriesService.getContents of package:github (a skeleton of it): the resource_name sink of the built-in rows
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-getcontents-ref/vuln', vuln);
  zooCase('F/dart/github-getcontents-ref/fixed', fixed);
  zooCase('F/dart/github-getcontents-ref/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-getcontents-ref-vuln'); // @source(dart-github-getcontents-ref-vuln)
  await repos.getContents(RepositorySlug('o', 'r'), 'README.md', ref: v); // @sink(dart-github-getcontents-ref-vuln)
}

Future<void> fixed() async { // @fixed
  final repos = RepositoriesService(GitHub());
  final v = cleanResource(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-github-getcontents-ref-fixed'))))));
  await repos.getContents(RepositorySlug('o', 'r'), 'README.md', ref: v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-getcontents-ref-decoy'); // @source(dart-github-getcontents-ref-decoy)
  await repos.getContents(RepositorySlug('o', 'r'), 'README.md', ref: 'main'); // @sink(none) @nosink(dart-github-getcontents-ref-decoy)
  sinkAny(v); // @sink(dart-github-getcontents-ref-decoy)
}
