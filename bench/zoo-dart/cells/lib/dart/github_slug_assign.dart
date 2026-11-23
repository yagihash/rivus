// @cell(F/dart/github-slug-assign)
// @shape: a value is assigned to the owner field of a RepositorySlug after it is made (package:github, a skeleton of it) and the slug goes into a service call: the sink rows are on RepositorySlug.new and .full, not on the field
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-slug-assign/vuln', vuln);
  zooCase('F/dart/github-slug-assign/fixed', fixed);
  zooCase('F/dart/github-slug-assign/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-assign-vuln'); // @source(dart-github-slug-assign-vuln) @miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)
  final slug = RepositorySlug('o', 'x');
  slug.owner = v;
  await repos.getReadme(slug); // @sink(dart-github-slug-assign-vuln)
}

Future<void> fixed() async { // @fixed
  final repos = RepositoriesService(GitHub());
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-github-slug-assign-fixed')))));
  final slug = RepositorySlug('o', 'x');
  slug.owner = v;
  await repos.getReadme(slug); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-assign-decoy'); // @source(dart-github-slug-assign-decoy)
  final slug = RepositorySlug('o', 'x');
  slug.owner = 'o2';
  await repos.getReadme(slug); // @sink(none) @nosink(dart-github-slug-assign-decoy)
  sinkAny(v); // @sink(dart-github-slug-assign-decoy)
}
