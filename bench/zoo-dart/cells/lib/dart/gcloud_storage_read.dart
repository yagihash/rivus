// @cell(F/dart/gcloud-storage-read)
// @elsewhere(go: F/pkg/storage/names)
// @shape: a value is the name of an object that Bucket.read of package:gcloud (a skeleton of it) reads: the resource_name sink of the built-in rows
import 'package:gcloud/storage.dart' as gcs;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/gcloud-storage-read/vuln', vuln);
  zooCase('F/dart/gcloud-storage-read/fixed', fixed);
  zooCase('F/dart/gcloud-storage-read/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final bucket = gcs.Storage(Object(), 'p').bucket('data');
  final v = src('dart-gcloud-storage-read-vuln'); // @source(dart-gcloud-storage-read-vuln)
  await bucket.read(v).drain<void>(); // @sink(dart-gcloud-storage-read-vuln)
}

Future<void> fixed() async { // @fixed
  final bucket = gcs.Storage(Object(), 'p').bucket('data');
  final v = cleanResource(cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-gcloud-storage-read-fixed'))))));
  await bucket.read(v).drain<void>(); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final bucket = gcs.Storage(Object(), 'p').bucket('data');
  final v = src('dart-gcloud-storage-read-decoy'); // @source(dart-gcloud-storage-read-decoy)
  await bucket.read('fixed.txt', offset: v.length).drain<void>(); // @sink(none) @nosink(dart-gcloud-storage-read-decoy)
  sinkAny(v); // @sink(dart-gcloud-storage-read-decoy)
}
