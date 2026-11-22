import 'dart:convert';

import 'package:gcloud/datastore.dart' as ds;
import 'package:gcloud/pubsub.dart' as pubsub;
import 'package:gcloud/storage.dart' as gcs;
import 'package:googleapis_auth/auth_io.dart' as auth;
import 'package:grpc/grpc.dart';

import '../generated/cloud.pbgrpc.dart';

const _project = 'example-project';

class GcloudService extends GcloudServiceBase {
  final Map<String, String> _names = {'a': 'bucket-a', 'b': 'bucket-b'};

  Future<gcs.Storage> _storage() async {
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: gcs.Storage.SCOPES);
    return gcs.Storage(client, _project);
  }

  @override
  Future<CloudReply> readObject(ServiceCall call, ObjectRequest request) async {
    // a: the object name of the request reaches the argument of the read.
    final storage = await _storage();
    final bucket = storage.bucket('data-bucket');
    final text = await bucket.read(request.object).transform(utf8.decoder).join();
    return CloudReply()..body = text;
  }

  @override
  Future<CloudReply> readObjectFixed(
      ServiceCall call, ObjectRequest request) async {
    // b: the key of the request is only used for a map lookup; the object is a constant.
    final storage = await _storage();
    final bucket = storage.bucket(_names[request.key] ?? 'bucket-a');
    final text = await bucket.read('fixed.txt').transform(utf8.decoder).join();
    return CloudReply()..body = text;
  }

  @override
  Future<CloudReply> bucketInfo(ServiceCall call, ObjectRequest request) async {
    // a: the bucket name of the request.
    final storage = await _storage();
    final info = await storage.bucketInfo(request.bucket);
    return CloudReply()..body = info.bucketName;
  }

  @override
  Future<CloudReply> publish(ServiceCall call, TopicRequest request) async {
    // a: the topic name of the request.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: pubsub.PubSub.SCOPES);
    final ps = pubsub.PubSub(client, _project);
    final topic = await ps.lookupTopic(request.topic);
    await topic.publishString('hello');
    return CloudReply()..status = '200';
  }

  @override
  Future<CloudReply> publishFixed(ServiceCall call, TopicRequest request) async {
    // b: the message of the request is the data; the topic is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: pubsub.PubSub.SCOPES);
    final ps = pubsub.PubSub(client, _project);
    final topic = await ps.lookupTopic('events');
    await topic.publishString(request.message);
    return CloudReply()..status = '200';
  }

  @override
  Future<CloudReply> query(ServiceCall call, SqlRequest request) async {
    // a: the kind of the request is the kind of the key that is looked up.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: ds.Datastore.Scopes);
    final store = ds.Datastore(client, _project);
    final key = ds.Key([ds.KeyElement(request.kind, 1)]);
    final found = await store.lookup([key]);
    return CloudReply()..body = '${found.length}';
  }

  @override
  Future<CloudReply> queryFixed(ServiceCall call, SqlRequest request) async {
    // b: the key of the request is only used for a map lookup; the kind is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: ds.Datastore.Scopes);
    final store = ds.Datastore(client, _project);
    final key = ds.Key([ds.KeyElement(_names[request.key] ?? 'Item', 1)]);
    final found = await store.lookup([key]);
    return CloudReply()..body = '${found.length}';
  }

  @override
  Future<CloudReply> filterValue(ServiceCall call, SqlRequest request) async {
    // b: the value of the request is the value of a filter, which the service binds; the kind is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: ds.Datastore.Scopes);
    final store = ds.Datastore(client, _project);
    final q = ds.Query(kind: 'Item', filters: [
      ds.Filter(ds.FilterRelation.Equal, 'name', request.value),
    ]);
    final page = await store.query(q);
    return CloudReply()..body = '${page.items.length}';
  }

  @override
  Future<CloudReply> pushEndpoint(ServiceCall call, TopicRequest request) async {
    // a: the URL of the request is where the subscription delivers its messages (SSRF).
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: pubsub.PubSub.SCOPES);
    final ps = pubsub.PubSub(client, _project);
    final sub = await ps.lookupSubscription('events-sub');
    await sub.updatePushConfiguration(Uri.parse(request.message));
    return CloudReply()..status = '200';
  }

  @override
  Future<CloudReply> pushEndpointFixed(
      ServiceCall call, TopicRequest request) async {
    // b: the key of the request is only used for a map lookup; the endpoint is a constant.
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: pubsub.PubSub.SCOPES);
    final ps = pubsub.PubSub(client, _project);
    final sub = await ps.lookupSubscription('events-sub');
    await sub.updatePushConfiguration(
        Uri.parse(_names[request.key] ?? 'https://example.invalid/push'));
    return CloudReply()..status = '200';
  }
}
