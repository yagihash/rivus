// This is a generated file - do not edit.
//
// Generated from jobs.proto.
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

import 'jobs.pb.dart' as $0;

export 'jobs.pb.dart';

@$pb.GrpcServiceName('bench.jobs.Jobs')
abstract class JobsServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.jobs.Jobs';

  JobsServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RunRequest, $0.RunReply>(
        'Run',
        run_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.RunRequest.fromBuffer(value),
        ($0.RunReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CancelRequest, $0.CancelReply>(
        'Cancel',
        cancel_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CancelRequest.fromBuffer(value),
        ($0.CancelReply value) => value.writeToBuffer()));
  }

  $async.Stream<$0.RunReply> run_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.RunRequest> $request) async* {
    yield* run($call, await $request);
  }

  $async.Future<$0.CancelReply> cancel_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.CancelRequest> $request) async {
    return cancel($call, await $request);
  }

  $async.Stream<$0.RunReply> run($grpc.ServiceCall call, $0.RunRequest request);
  $async.Future<$0.CancelReply> cancel($grpc.ServiceCall call, $0.CancelRequest request);
}
