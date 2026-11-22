import 'dart:convert';

import 'package:googleapis/bigquery/v2.dart' as bq;
import 'package:googleapis/datastore/v1.dart' as ds;
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

  @override
  Future<CloudReply> runGql(ServiceCall call, SqlRequest request) async {
    // a: the GQL text of the request.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [ds.DatastoreApi.datastoreScope]);
    final api = ds.DatastoreApi(client);
    final res = await api.projects.runQuery(
        ds.RunQueryRequest(gqlQuery: ds.GqlQuery(queryString: request.sql)),
        _project);
    return CloudReply()..body = '${res.batch}';
  }

  @override
  Future<CloudReply> runGqlFixed(ServiceCall call, SqlRequest request) async {
    // b: the value of the request is a binding of a constant GQL text.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [ds.DatastoreApi.datastoreScope]);
    final api = ds.DatastoreApi(client);
    final query = ds.GqlQuery(
        queryString: 'SELECT * FROM Item WHERE name = @name',
        namedBindings: {
          'name': ds.GqlQueryParameter(value: ds.Value(stringValue: request.value)),
        });
    final res = await api.projects
        .runQuery(ds.RunQueryRequest(gqlQuery: query), _project);
    return CloudReply()..body = '${res.batch}';
  }

  @override
  Future<CloudReply> modifyPush(ServiceCall call, TopicRequest request) async {
    // a: the URL of the request is the push endpoint of the subscription (SSRF).
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    await api.projects.subscriptions.modifyPushConfig(
        pubsub.ModifyPushConfigRequest(
            pushConfig: pubsub.PushConfig(pushEndpoint: request.message)),
        'projects/$_project/subscriptions/events-sub');
    return CloudReply()..status = '200';
  }

  @override
  Future<CloudReply> modifyPushFixed(
      ServiceCall call, TopicRequest request) async {
    // b: the key of the request is only used for a map lookup; the endpoint is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    final endpoint = _names[request.key] == null
        ? 'https://example.invalid/a'
        : 'https://example.invalid/b';
    await api.projects.subscriptions.modifyPushConfig(
        pubsub.ModifyPushConfigRequest(
            pushConfig: pubsub.PushConfig(pushEndpoint: endpoint)),
        'projects/$_project/subscriptions/events-sub');
    return CloudReply()..status = '200';
  }

  @override
  Future<CloudReply> modifyPushName(ServiceCall call, TopicRequest request) async {
    // a, a known miss (#140): the name of the subscription is the request's; the row of this call is the push endpoint.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: [pubsub.PubsubApi.pubsubScope]);
    final api = pubsub.PubsubApi(client);
    await api.projects.subscriptions.modifyPushConfig(
        pubsub.ModifyPushConfigRequest(
            pushConfig: pubsub.PushConfig(pushEndpoint: 'https://example.invalid/push')),
        'projects/$_project/subscriptions/${request.topic}');
    return CloudReply()..status = '200';
  }
}
