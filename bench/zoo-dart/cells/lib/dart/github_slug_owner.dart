// @cell(F/dart/github-slug-owner)
// @elsewhere(go: F/pkg/go-github/repos)
// @shape: a value goes into the owner of a RepositorySlug of package:github (a skeleton of it): the resource_name sink of the built-in rows
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-slug-owner/vuln', vuln);
  zooCase('F/dart/github-slug-owner/fixed', fixed);
  zooCase('F/dart/github-slug-owner/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-owner-vuln'); // @source(dart-github-slug-owner-vuln)
  await repos.getReadme(RepositorySlug(v, 'r')); // @sink(dart-github-slug-owner-vuln)
}

Future<void> fixed() async { // @fixed
  final repos = RepositoriesService(GitHub());
  final v = cleanResource(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-github-slug-owner-fixed'))))));
  await repos.getReadme(RepositorySlug(v, 'r')); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-owner-decoy'); // @source(dart-github-slug-owner-decoy)
  await repos.getReadme(RepositorySlug('o', 'r')); // @sink(none) @nosink(dart-github-slug-owner-decoy)
  sinkAny(v); // @sink(dart-github-slug-owner-decoy)
}
