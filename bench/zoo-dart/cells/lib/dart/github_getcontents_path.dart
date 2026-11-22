// @cell(F/dart/github-getcontents-path)
// @shape: a value goes into the path of RepositoriesService.getContents of package:github (a skeleton of it): the resource_name sink of the built-in rows
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-getcontents-path/vuln', vuln);
  zooCase('F/dart/github-getcontents-path/fixed', fixed);
  zooCase('F/dart/github-getcontents-path/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-getcontents-path-vuln'); // @source(dart-github-getcontents-path-vuln)
  await repos.getContents(RepositorySlug('o', 'r'), v); // @sink(dart-github-getcontents-path-vuln)
}

Future<void> fixed() async { // @fixed
  final repos = RepositoriesService(GitHub());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-github-getcontents-path-fixed')))));
  await repos.getContents(RepositorySlug('o', 'r'), v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-getcontents-path-decoy'); // @source(dart-github-getcontents-path-decoy)
  await repos.getContents(RepositorySlug('o', 'r'), 'README.md'); // @sink(none) @nosink(dart-github-getcontents-path-decoy)
  sinkAny(v); // @sink(dart-github-getcontents-path-decoy)
}
