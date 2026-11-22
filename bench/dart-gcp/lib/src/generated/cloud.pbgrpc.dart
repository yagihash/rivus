// This is a generated file - do not edit.
//
// Generated from cloud.proto.
// (Written by hand in the shape of protoc_plugin's output; protoc was not run.)

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'cloud.pb.dart' as $0;

export 'cloud.pb.dart';

@$pb.GrpcServiceName('bench.cloud.Gcloud')
abstract class GcloudServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.cloud.Gcloud';

  GcloudServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'ReadObject',
        readObject_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'ReadObjectFixed',
        readObjectFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'BucketInfo',
        bucketInfo_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'Publish',
        publish_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'PublishFixed',
        publishFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SqlRequest, $0.CloudReply>(
        'Query',
        query_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SqlRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SqlRequest, $0.CloudReply>(
        'QueryFixed',
        queryFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SqlRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.CloudReply> readObject_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return readObject($call, await $request);
  }

  $async.Future<$0.CloudReply> readObjectFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return readObjectFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> bucketInfo_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return bucketInfo($call, await $request);
  }

  $async.Future<$0.CloudReply> publish_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return publish($call, await $request);
  }

  $async.Future<$0.CloudReply> publishFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return publishFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> query_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.SqlRequest> $request) async {
    return query($call, await $request);
  }

  $async.Future<$0.CloudReply> queryFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.SqlRequest> $request) async {
    return queryFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> readObject($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> readObjectFixed($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> bucketInfo($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> publish($grpc.ServiceCall call, $0.TopicRequest request);
  $async.Future<$0.CloudReply> publishFixed($grpc.ServiceCall call, $0.TopicRequest request);
  $async.Future<$0.CloudReply> query($grpc.ServiceCall call, $0.SqlRequest request);
  $async.Future<$0.CloudReply> queryFixed($grpc.ServiceCall call, $0.SqlRequest request);
}

@$pb.GrpcServiceName('bench.cloud.Apis')
abstract class ApisServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.cloud.Apis';

  ApisServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'GetObject',
        getObject_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'GetObjectFixed',
        getObjectFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'PublishTopic',
        publishTopic_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'PublishTopicFixed',
        publishTopicFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SqlRequest, $0.CloudReply>(
        'RunQuery',
        runQuery_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SqlRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SqlRequest, $0.CloudReply>(
        'RunQueryFixed',
        runQueryFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SqlRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.CloudReply> getObject_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return getObject($call, await $request);
  }

  $async.Future<$0.CloudReply> getObjectFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return getObjectFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> publishTopic_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return publishTopic($call, await $request);
  }

  $async.Future<$0.CloudReply> publishTopicFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return publishTopicFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> runQuery_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.SqlRequest> $request) async {
    return runQuery($call, await $request);
  }

  $async.Future<$0.CloudReply> runQueryFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.SqlRequest> $request) async {
    return runQueryFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> getObject($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> getObjectFixed($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> publishTopic($grpc.ServiceCall call, $0.TopicRequest request);
  $async.Future<$0.CloudReply> publishTopicFixed($grpc.ServiceCall call, $0.TopicRequest request);
  $async.Future<$0.CloudReply> runQuery($grpc.ServiceCall call, $0.SqlRequest request);
  $async.Future<$0.CloudReply> runQueryFixed($grpc.ServiceCall call, $0.SqlRequest request);
}

@$pb.GrpcServiceName('bench.cloud.Json')
abstract class JsonServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.cloud.Json';

  JsonServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'InsertBucket',
        insertBucket_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'PatchObject',
        patchObject_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SqlRequest, $0.CloudReply>(
        'InsertJob',
        insertJob_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SqlRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'CreateTopic',
        createTopic_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TopicRequest, $0.CloudReply>(
        'Mask',
        mask_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TopicRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.CloudReply> insertBucket_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return insertBucket($call, await $request);
  }

  $async.Future<$0.CloudReply> patchObject_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return patchObject($call, await $request);
  }

  $async.Future<$0.CloudReply> insertJob_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.SqlRequest> $request) async {
    return insertJob($call, await $request);
  }

  $async.Future<$0.CloudReply> createTopic_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return createTopic($call, await $request);
  }

  $async.Future<$0.CloudReply> mask_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TopicRequest> $request) async {
    return mask($call, await $request);
  }

  $async.Future<$0.CloudReply> insertBucket($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> patchObject($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> insertJob($grpc.ServiceCall call, $0.SqlRequest request);
  $async.Future<$0.CloudReply> createTopic($grpc.ServiceCall call, $0.TopicRequest request);
  $async.Future<$0.CloudReply> mask($grpc.ServiceCall call, $0.TopicRequest request);
}

@$pb.GrpcServiceName('bench.cloud.ManyApis')
abstract class ManyApisServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.cloud.ManyApis';

  ManyApisServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'Inspect',
        inspect_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.CloudReply> inspect_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return inspect($call, await $request);
  }

  $async.Future<$0.CloudReply> inspect($grpc.ServiceCall call, $0.ObjectRequest request);
}

@$pb.GrpcServiceName('bench.cloud.CloudStorage')
abstract class CloudStorageServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.cloud.CloudStorage';

  CloudStorageServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'Download',
        download_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'DownloadFixed',
        downloadFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'Upload',
        upload_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObjectRequest, $0.CloudReply>(
        'UploadFixed',
        uploadFixed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ObjectRequest.fromBuffer(value),
        ($0.CloudReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.CloudReply> download_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return download($call, await $request);
  }

  $async.Future<$0.CloudReply> downloadFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return downloadFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> upload_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return upload($call, await $request);
  }

  $async.Future<$0.CloudReply> uploadFixed_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ObjectRequest> $request) async {
    return uploadFixed($call, await $request);
  }

  $async.Future<$0.CloudReply> download($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> downloadFixed($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> upload($grpc.ServiceCall call, $0.ObjectRequest request);
  $async.Future<$0.CloudReply> uploadFixed($grpc.ServiceCall call, $0.ObjectRequest request);
}
