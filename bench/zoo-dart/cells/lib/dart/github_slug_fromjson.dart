// @cell(F/dart/github-slug-fromjson)
// @shape: a value goes into the owner of a RepositorySlug that is made by RepositorySlug.fromJson (package:github, a skeleton of it) and the slug goes into a service call: the sink rows are on RepositorySlug.new and .full, not on fromJson
import 'package:github/github.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/github-slug-fromjson/vuln', vuln);
  zooCase('F/dart/github-slug-fromjson/fixed', fixed);
  zooCase('F/dart/github-slug-fromjson/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-fromjson-vuln'); // @source(dart-github-slug-fromjson-vuln) @miss(unmodeled_library_call) @ref(model-dart-github-repositoryslug-miss)
  final slug = RepositorySlug.fromJson({'owner': v, 'name': 'x'});
  await repos.getReadme(slug); // @sink(dart-github-slug-fromjson-vuln)
}

Future<void> fixed() async { // @fixed
  final repos = RepositoriesService(GitHub());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-github-slug-fromjson-fixed')))));
  final slug = RepositorySlug.fromJson({'owner': v, 'name': 'x'});
  await repos.getReadme(slug); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final repos = RepositoriesService(GitHub());
  final v = src('dart-github-slug-fromjson-decoy'); // @source(dart-github-slug-fromjson-decoy)
  final slug = RepositorySlug.fromJson({'owner': 'o', 'name': 'x'});
  await repos.getReadme(slug); // @sink(none) @nosink(dart-github-slug-fromjson-decoy)
  sinkAny(v); // @sink(dart-github-slug-fromjson-decoy)
}
