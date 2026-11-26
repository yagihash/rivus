// @cell(F/dart/archive-file-name)
// @na(go: the model of package:archive, a Dart package; the Go side is measured in zoo-go-pkg (archive: archive/tar and archive/zip), not by a cell of the same name)
// @shape: the name of an entry of an archive (and the target of a link) is written by whoever made the archive, so it is a source of package:archive (a skeleton of it): the entry point of zip slip. A name that went through the path sanitizer is not a finding, and a field that is not a name (the size, the comment) is not a source
import 'package:archive/archive.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/archive-file-name/vuln', vuln);
  zooCase('F/dart/archive-file-name/fixed', fixed);
  zooCase('F/dart/archive-file-name/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  {
    final f = ArchiveFile(typed('dart-archive-file-name-vuln-name'), 0, []);
    sinkPath('/srv/base', f.name); // @source(dart-archive-file-name-vuln-name) @sink(dart-archive-file-name-vuln-name)
  }
  {
    final f = ArchiveFile.symlink('x', typed('dart-archive-file-name-vuln-link'));
    sinkPath('/srv/base', f.symbolicLink!); // @source(dart-archive-file-name-vuln-link) @sink(dart-archive-file-name-vuln-link)
  }
}

Future<void> fixed() async { // @fixed
  final f = ArchiveFile.symlink(typed('dart-archive-file-name-fixed'), typed('dart-archive-file-name-fixed'));
  sinkPath('/srv/base', cleanPath(f.name)); // @sink(none)
  sinkPath('/srv/base', cleanPath(f.symbolicLink!)); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final f = ArchiveFile(typed('dart-archive-file-name-decoy'), 0, []);
  f.comment = 'note';
  final v = f.name; // @source(dart-archive-file-name-decoy)
  sinkPath('/srv/base', 'constant'); // @sink(none) @nosink(dart-archive-file-name-decoy)
  sinkPath('/srv/base', f.comment!); // @sink(none) @nosink(dart-archive-file-name-decoy)
  sinkPath('/srv/base', v); // @sink(dart-archive-file-name-decoy)
}
