// the shape of tar 2.x: the header is a sealed class with the name and the target of a link as getters
sealed class TarHeader {
  String get name;
  String? get linkName;
  int get size;
  String? get userName;

  factory TarHeader({required String name, String? linkName, int size = 0, String? userName}) =>
      _Header(name: name, linkName: linkName, size: size, userName: userName);
}

class _Header implements TarHeader {
  _Header({required this.name, this.linkName, this.size = 0, this.userName});

  @override
  final String name;
  @override
  final String? linkName;
  @override
  final int size;
  @override
  final String? userName;
}
