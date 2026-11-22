// This is a generated file - do not edit.
//
// Generated from reports.proto.
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

import 'reports.pb.dart' as $0;

export 'reports.pb.dart';

@$pb.GrpcServiceName('bench.reports.Reports')
abstract class ReportsServiceBase extends $grpc.Service {
  $core.String get $name => 'bench.reports.Reports';

  ReportsServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.FetchRequest, $0.FetchReply>(
        'Fetch',
        fetch_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.FetchRequest.fromBuffer(value),
        ($0.FetchReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LookupRequest, $0.LookupReply>(
        'Lookup',
        lookup_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LookupRequest.fromBuffer(value),
        ($0.LookupReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.FetchReply> fetch_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.FetchRequest> $request) async {
    return fetch($call, await $request);
  }

  $async.Future<$0.LookupReply> lookup_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.LookupRequest> $request) async {
    return lookup($call, await $request);
  }

  $async.Future<$0.FetchReply> fetch($grpc.ServiceCall call, $0.FetchRequest request);
  $async.Future<$0.LookupReply> lookup($grpc.ServiceCall call, $0.LookupRequest request);
}
