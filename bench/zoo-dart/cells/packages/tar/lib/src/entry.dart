import 'header.dart';

final class TarEntry {
  final TarHeader header;

  TarEntry(this.header);

  String get name => header.name;
  int get size => header.size;
}
