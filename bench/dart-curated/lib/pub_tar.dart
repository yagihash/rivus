// The shape of dart-lang/pub's extractTarGz (bench/dart-pub, GHSA-q739-79rh-vmvp): the name of
// the entries of a tar archive is read from a reader in a `while (await reader.moveNext())` loop, the
// destination path is made of it by joining the parts of the name, the entry type is switched on, and
// the path goes to the file helpers in every case. The reader and the entry are the project's own
// classes here (the real ones are in package:tar); the source is the getter of the name (rules.yaml).

class TarHeader {
  final String name;
  final String? linkName;
  final int mode;
  TarHeader(this.name, this.linkName, this.mode);
}

enum TypeFlag { reg, dir, symlink, link }

class TarEntry {
  final TarHeader header;
  final Stream<List<int>> contents;
  final TypeFlag type;
  TarEntry(this.header, this.contents, this.type);

  // a source (rules.yaml): the name is what the author of the package wrote
  String get name => header.name;
}

class TarReader {
  final Stream<List<int>> _stream;
  TarReader(this._stream);
  late TarEntry _current;
  TarEntry get current => _current;
  Future<bool> moveNext() async {
    _current = TarEntry(TarHeader('a/b', null, 0), _stream, TypeFlag.reg);
    return true;
  }

  Future<void> cancel() async {}
}

// the file helpers of io.dart: the sinks (rules.yaml)
void ensureDir(String path) {}
void deleteIfLink(String path) {}
Future<void> createFileFromStream(Stream<List<int>> stream, String path) async {}
void chmod(int mode, String path) {}
void createSymlink(String target, String link) {}

String joinAll(Iterable<String> parts) => parts.join('/');
String dirname(String p) => p.substring(0, p.lastIndexOf('/') + 1);

// the loop of extractTarGz, without the check of the path (what the fix adds)
Future<void> extractTarGz(Stream<List<int>> stream, String destination) async {
  final reader = TarReader(stream);
  final paths = <String>{};
  while (await reader.moveNext()) {
    final entry = reader.current;
    final filePath = joinAll([destination, ...entry.name.split('/')]); // @source(pub_name)
    if (!paths.add(filePath)) {
      await reader.cancel();
      throw FormatException('Tar file contained duplicate path');
    }
    final parentDirectory = dirname(filePath);
    switch (entry.type) {
      case TypeFlag.dir:
        ensureDir(filePath); // @sink(pub_name)
        break;
      case TypeFlag.reg:
        deleteIfLink(filePath); // @sink(pub_name)
        ensureDir(parentDirectory); // @sink(pub_name)
        await createFileFromStream(entry.contents, filePath); // @sink(pub_name)
        final mode = entry.header.mode & 0x49;
        if (mode != 0) {
          chmod(mode, filePath); // @sink(pub_name)
        }
        break;
      case TypeFlag.symlink:
        final resolved = joinAll([parentDirectory, ...entry.header.linkName!.split('/')]);
        ensureDir(parentDirectory); // @sink(pub_name)
        createSymlink(resolved, filePath); // @sink(pub_name)
        break;
      case TypeFlag.link:
        final from = joinAll([destination, entry.header.linkName!]);
        ensureDir(parentDirectory); // @sink(pub_name)
        createSymlink(from, filePath); // @sink(pub_name)
        break;
    }
  }
}
