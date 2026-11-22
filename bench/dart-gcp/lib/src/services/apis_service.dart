import 'dart:convert';

import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:googleapis/pubsub/v1.dart' as pubsub;
import 'package:googleapis/storage/v1.dart' as storage;
import 'package:googleapis_auth/auth_io.dart' as auth;
import 'package:grpc/grpc.dart';

import '../generated/cloud.pbgrpc.dart';

const _project = 'example-project';

class ApisService extends ApisServiceBase {
  final Map<String, String> _names = {'a': 'bucket-a', 'b': 'bucket-b'};

  @override
  Future<CloudReply> getObject(ServiceCall call, ObjectRequest request) async {
    // a: the bucket and the object name of the request.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [storage.StorageApi.devstorageReadOnlyScope]);
    final api = storage.StorageApi(client);
    final obj = await api.objects.get(request.bucket, request.object);
    return CloudReply()..body = '$obj';
  }

  @override
  Future<CloudReply> getObjectFixed(
      ServiceCall call, ObjectRequest request) async {
    // b: the key is only used for a map lookup; the names are constants.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [storage.StorageApi.devstorageReadOnlyScope]);
    final api = storage.StorageApi(client);
    final obj = await api.objects.get(_names[request.key] ?? 'bucket-a', 'fixed.txt');
    return CloudReply()..body = '$obj';
  }

  @override
  Future<CloudReply> publishTopic(ServiceCall call, TopicRequest request) async {
    // a: the topic name of the request.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    final msg = pubsub.PubsubMessage(data: base64Encode(utf8.encode('hello')));
    final res = await api.projects.topics.publish(
        pubsub.PublishRequest(messages: [msg]),
        'projects/$_project/topics/${request.topic}');
    return CloudReply()..body = '${res.messageIds}';
  }

  @override
  Future<CloudReply> publishTopicFixed(
      ServiceCall call, TopicRequest request) async {
    // b: the message of the request is the data; the topic is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    final msg =
        pubsub.PubsubMessage(data: base64Encode(utf8.encode(request.message)));
    final res = await api.projects.topics.publish(
        pubsub.PublishRequest(messages: [msg]),
        'projects/$_project/topics/events');
    return CloudReply()..body = '${res.messageIds}';
  }

  @override
  Future<CloudReply> runQuery(ServiceCall call, SqlRequest request) async {
    // a: the SQL text of the request.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [bq.BigqueryApi.bigqueryScope]);
    final api = bq.BigqueryApi(client);
    final res = await api.jobs
        .query(bq.QueryRequest(query: request.sql, useLegacySql: false), _project);
    return CloudReply()..body = '${res.totalRows}';
  }

  @override
  Future<CloudReply> runQueryFixed(ServiceCall call, SqlRequest request) async {
    // b: the key is only used for a map lookup; the query is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [bq.BigqueryApi.bigqueryScope]);
    final api = bq.BigqueryApi(client);
    final sql = _names[request.key] == null
        ? 'SELECT 1'
        : 'SELECT name FROM dataset.items LIMIT 10';
    final res = await api.jobs.query(bq.QueryRequest(query: sql), _project);
    return CloudReply()..body = '${res.totalRows}';
  }
}
