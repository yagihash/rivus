// The skeleton of the Storage API of `package:gcloud`: an interface whose methods take the name first. It is real
// code, not a declaration: the harness runs it and it records the name that a request would be made for as what
// reached the sink; rivus does not read it as a dependency's body where the built-in rows stand for it.
import 'package:zoo_mark/zoo_mark.dart';

abstract interface class Storage {
  factory Storage(Object client, String project) => _SkeletonStorage();

  Bucket bucket(String bucketName);
}

abstract interface class Bucket {
  Stream<List<int>> read(String objectName, {int? offset, int? length});
}

class _SkeletonStorage implements Storage {
  @override
  Bucket bucket(String bucketName) => _SkeletonBucket();
}

class _SkeletonBucket implements Bucket {
  @override
  Stream<List<int>> read(String objectName, {int? offset, int? length}) {
    recordAny(objectName);
    return const Stream<List<int>>.empty();
  }
}
