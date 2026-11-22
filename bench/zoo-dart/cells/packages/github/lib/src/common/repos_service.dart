// The skeleton of `RepositoriesService` of package:github: it records the resource that a call reaches (the slug, the path
// and the ref, as the real one puts them into the URL) and returns an empty value.
import 'package:zoo_mark/zoo_mark.dart';

import 'github.dart';
import 'model/repos.dart';
import 'model/repos_contents.dart';

class RepositoriesService {
  RepositoriesService(this.github);

  final GitHub github;

  Future<RepositoryContents> getContents(RepositorySlug slug, String path, {String? ref}) async {
    recordAny('/repos/${slug.fullName}/contents/$path${ref == null ? '' : '?ref=$ref'}');
    return RepositoryContents();
  }

  Future<GitHubFile> getReadme(RepositorySlug slug, {String? ref}) async {
    recordAny('/repos/${slug.fullName}/readme${ref == null ? '' : '?ref=$ref'}');
    return GitHubFile();
  }
}
