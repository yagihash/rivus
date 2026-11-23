// @cell(F/dart/tar-entry-name)
// @shape: the name of an entry of a tar archive (and the target of a link) is written by whoever made the archive, so it is a source of package:tar (a skeleton of it): the entry point of zip slip. A name that went through the path sanitizer is not a finding, and a field that is not a name (the size, the user name) is not a source
import 'package:tar/tar.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/tar-entry-name/vuln', vuln);
  zooCase('F/dart/tar-entry-name/fixed', fixed);
  zooCase('F/dart/tar-entry-name/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  {
    final entry = TarEntry(TarHeader(name: typed('dart-tar-entry-name-vuln-entry')));
    sinkPath('/srv/base', entry.name); // @source(dart-tar-entry-name-vuln-entry) @sink(dart-tar-entry-name-vuln-entry)
  }
  {
    final entry = TarEntry(TarHeader(name: typed('dart-tar-entry-name-vuln-header')));
    sinkPath('/srv/base', entry.header.name); // @source(dart-tar-entry-name-vuln-header) @sink(dart-tar-entry-name-vuln-header)
  }
  {
    final entry = TarEntry(TarHeader(name: 'x', linkName: typed('dart-tar-entry-name-vuln-link')));
    sinkPath('/srv/base', entry.header.linkName!); // @source(dart-tar-entry-name-vuln-link) @sink(dart-tar-entry-name-vuln-link)
  }
}

Future<void> fixed() async { // @fixed
  final entry = TarEntry(TarHeader(name: typed('dart-tar-entry-name-fixed'), linkName: typed('dart-tar-entry-name-fixed')));
  sinkPath('/srv/base', cleanPath(entry.name)); // @sink(none)
  sinkPath('/srv/base', cleanPath(entry.header.name)); // @sink(none)
  sinkPath('/srv/base', cleanPath(entry.header.linkName!)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final entry = TarEntry(TarHeader(name: typed('dart-tar-entry-name-decoy'), userName: 'root'));
  final v = entry.name; // @source(dart-tar-entry-name-decoy)
  sinkPath('/srv/base', 'constant'); // @sink(none) @nosink(dart-tar-entry-name-decoy)
  sinkPath('/srv/base', entry.header.userName!); // @sink(none) @nosink(dart-tar-entry-name-decoy)
  sinkPath('/srv/base', v); // @sink(dart-tar-entry-name-decoy)
}
