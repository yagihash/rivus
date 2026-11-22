import 'dart:convert';

import 'package:google_cloud_storage/google_cloud_storage.dart';
import 'package:grpc/grpc.dart';

import '../generated/cloud.pbgrpc.dart';

class CloudStorageService extends CloudStorageServiceBase {
  final Map<String, String> _names = {'a': 'bucket-a', 'b': 'bucket-b'};
  final Storage _storage = Storage();

  @override
  Future<CloudReply> download(ServiceCall call, ObjectRequest request) async {
    // a: the bucket and the object name of the request.
    final bytes = await _storage.downloadObject(request.bucket, request.object);
    return CloudReply()..body = utf8.decode(bytes);
  }

  @override
  Future<CloudReply> downloadFixed(
      ServiceCall call, ObjectRequest request) async {
    // b: the key is only used for a map lookup; the names are constants.
    final bytes = await _storage.downloadObject(
        _names[request.key] ?? 'bucket-a', 'fixed.txt');
    return CloudReply()..body = utf8.decode(bytes);
  }

  @override
  Future<CloudReply> upload(ServiceCall call, ObjectRequest request) async {
    // a: the object name of the request; the content is also the request's.
    final md = await _storage.uploadObjectFromString(
        'upload-bucket', request.object, request.content);
    return CloudReply()..body = md.name ?? '';
  }

  @override
  Future<CloudReply> uploadFixed(ServiceCall call, ObjectRequest request) async {
    // b: only the content comes from the request; the bucket and the object are constants.
    final md = await _storage.uploadObjectFromString(
        'upload-bucket', 'fixed.txt', request.content);
    return CloudReply()..body = md.name ?? '';
  }
}
