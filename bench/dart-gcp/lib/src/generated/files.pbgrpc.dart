// This is a generated file - do not edit.
//
// Generated from files.proto.
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

import 'files.pb.dart' as $0;

export 'files.pb.dart';

@$pb.GrpcServiceName('bench.files.Files')
abstract class FilesServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.files.Files';

  FilesServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ReadRequest, $0.ReadReply>(
        'Read',
        read_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ReadRequest.fromBuffer(value),
        ($0.ReadReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WriteRequest, $0.WriteReply>(
        'Write',
        write_Pre,
        true,
        false,
        ($core.List<$core.int> value) => $0.WriteRequest.fromBuffer(value),
        ($0.WriteReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StatRequest, $0.StatReply>(
        'Stat',
        stat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.StatRequest.fromBuffer(value),
        ($0.StatReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.ReadReply> read_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.ReadRequest> $request) async {
    return read($call, await $request);
  }

  $async.Future<$0.WriteReply> write_Pre(
      $grpc.ServiceCall $call, $async.Stream<$0.WriteRequest> $request) async {
    return write($call, $request);
  }

  $async.Future<$0.StatReply> stat_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.StatRequest> $request) async {
    return stat($call, await $request);
  }

  $async.Future<$0.ReadReply> read($grpc.ServiceCall call, $0.ReadRequest request);
  $async.Future<$0.WriteReply> write($grpc.ServiceCall call, $async.Stream<$0.WriteRequest> request);
  $async.Future<$0.StatReply> stat($grpc.ServiceCall call, $0.StatRequest request);
}
