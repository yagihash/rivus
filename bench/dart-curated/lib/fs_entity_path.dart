import 'dart:io' as io;

// Needs package:file to be in the program, even if nothing here calls it: its entities implement the one of
// dart:io, so `path` of a dart:io entity gets a candidate whose body only forwards to another dart:io entity.
import 'package:file/file.dart' as pf;

import 'lib.dart';

// The shape of dart_services' _copyPath: the path of an entity is the argument of a Directory or a Link.
// The type of the entity is dart:io's FileSystemEntity. Before the model of dart:io#FileSystemEntity.path, a package
// that holds package:file lost the flow without a word (docs/issues/213): the candidate ForwardingFileSystemEntity.path
// has no output (its body is dart:io's, which is not read). Without the dependency the flow was found.

pf.LocalEntity<io.File>? _unused;

void fs_entity_dir() {
  final io.FileSystemEntity e = io.File.fromUri(Uri.file(source())); // @source(fs-entity-dir)
  io.Directory(e.path); // @sink(fs-entity-dir)
}

void fs_entity_link() {
  final io.FileSystemEntity e = io.File.fromUri(Uri.file(source())); // @source(fs-entity-link)
  io.Link(e.path); // @sink(fs-entity-link)
}
