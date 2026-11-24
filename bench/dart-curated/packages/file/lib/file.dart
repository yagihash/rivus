import 'dart:io' as io;

// The shape of package:file: its entity implements the one of dart:io, and the forwarding entity answers
// `path` with the path of its delegate, which is a dart:io entity (the body is in the SDK).
abstract class FileSystemEntity implements io.FileSystemEntity {}

abstract class ForwardingFileSystemEntity<T extends FileSystemEntity, D extends io.FileSystemEntity>
    implements FileSystemEntity {
  D get delegate;

  @override
  String get path => delegate.path;
}

// as LocalFile of the real package: the delegate is given to the constructor
class LocalEntity<D extends io.FileSystemEntity> extends ForwardingFileSystemEntity<LocalEntity<D>, D> {
  LocalEntity(this.delegate);

  @override
  final D delegate;

  // the rest of the interface is not kept in this skeleton
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
