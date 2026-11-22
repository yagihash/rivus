import 'dart:convert';

import 'package:google_cloud_protobuf/protobuf.dart' as gpb;
import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:googleapis/pubsub/v1.dart' as pubsub;
import 'package:googleapis/storage/v1.dart' as storage;
import 'package:googleapis_auth/auth_io.dart' as auth;
import 'package:grpc/grpc.dart';

import '../generated/cloud.pbgrpc.dart';

/// Handlers that take a JSON text of the request and give the decoded Map to the
/// `fromJson` constructors of the client packages, and the `toJson` result back.
class JsonService extends JsonServiceBase {
  @override
  Future<CloudReply> insertBucket(ServiceCall call, ObjectRequest request) async {
    // a: the whole bucket resource comes from the JSON of the request.
    final map = jsonDecode(request.content) as Map<String, dynamic>;
    final bucket = storage.Bucket.fromJson(map);
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [storage.StorageApi.devstorageFullControlScope]);
    final api = storage.StorageApi(client);
    final created = await api.buckets.insert(bucket, 'example-project');
    return CloudReply()..body = jsonEncode(created.toJson());
  }

  @override
  Future<CloudReply> patchObject(ServiceCall call, ObjectRequest request) async {
    // a: the object resource and its name come from the request.
    final map = jsonDecode(request.content) as Map<String, dynamic>;
    final obj = storage.Object.fromJson(map);
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [storage.StorageApi.devstorageFullControlScope]);
    final api = storage.StorageApi(client);
    final patched = await api.objects.patch(obj, request.bucket, request.object);
    return CloudReply()..body = jsonEncode(patched.toJson());
  }

  @override
  Future<CloudReply> insertJob(ServiceCall call, SqlRequest request) async {
    // a: the whole job resource comes from the JSON of the request.
    final map = jsonDecode(request.value) as Map<String, dynamic>;
    final job = bq.Job.fromJson(map);
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [bq.BigqueryApi.bigqueryScope]);
    final api = bq.BigqueryApi(client);
    final created = await api.jobs.insert(job, 'example-project');
    return CloudReply()..body = jsonEncode(created.toJson());
  }

  @override
  Future<CloudReply> createTopic(ServiceCall call, TopicRequest request) async {
    // a: the topic resource and its name come from the request.
    final map = jsonDecode(request.message) as Map<String, dynamic>;
    final topic = pubsub.Topic.fromJson(map);
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    final created = await api.projects.topics
        .create(topic, 'projects/example-project/topics/${request.topic}');
    return CloudReply()..body = jsonEncode(created.toJson());
  }

  @override
  Future<CloudReply> mask(ServiceCall call, TopicRequest request) async {
    // a: the field mask and the timestamp of the protobuf package come from the JSON of the request.
    final json = jsonDecode(request.message);
    final fm = gpb.FieldMask.fromJson(json);
    final ts = gpb.Timestamp.fromJson(request.key);
    return CloudReply()..body = '${jsonEncode(fm.toJson())} ${ts.toJson()}';
  }
}
