class RepositorySlug {
  RepositorySlug(this.owner, this.name);

  factory RepositorySlug.full(String f) {
    final split = f.split('/');
    return RepositorySlug(split[0], (split..removeAt(0)).join('/'));
  }

  // As the real one: `fromJson` is made by the code generator, which calls the public constructor.
  factory RepositorySlug.fromJson(Map<String, dynamic> json) => _slugFromJson(json);

  String owner;
  String name;

  String get fullName => '$owner/$name';
}

RepositorySlug _slugFromJson(Map<String, dynamic> json) => RepositorySlug(
      json['owner'] as String,
      json['name'] as String,
    );
