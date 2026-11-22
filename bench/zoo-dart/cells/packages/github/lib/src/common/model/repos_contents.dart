class GitHubFile {
  GitHubFile({this.path, this.content});

  String? path;
  String? content;

  // the real one decodes the base64 of `content`; the skeleton hands it on
  String get text => content ?? '';
}

class RepositoryContents {
  RepositoryContents({this.file, this.tree});

  GitHubFile? file;
  List<GitHubFile>? tree;
}

class CreateFile {
  CreateFile({this.path, this.content, this.message, this.branch});

  String? path;
  String? message;
  String? content;
  String? branch;
}
