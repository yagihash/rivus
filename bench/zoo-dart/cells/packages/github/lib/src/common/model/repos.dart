class RepositorySlug {
  RepositorySlug(this.owner, this.name);

  factory RepositorySlug.full(String f) {
    final split = f.split('/');
    return RepositorySlug(split[0], (split..removeAt(0)).join('/'));
  }

  String owner;
  String name;

  String get fullName => '$owner/$name';
}
