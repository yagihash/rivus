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

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Message ReadRequest.
class ReadRequest extends $pb.GeneratedMessage {
  factory ReadRequest({
    $core.String? path,
  }) {
    final result = create();
    if (path != null) result.path = path;
    return result;
  }

  ReadRequest._();

  factory ReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRequest copyWith(void Function(ReadRequest) updates) =>
      super.copyWith((message) => updates(message as ReadRequest)) as ReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadRequest create() => ReadRequest._();
  @$core.override
  ReadRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadRequest>(create);
  static ReadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

/// Message ReadReply.
class ReadReply extends $pb.GeneratedMessage {
  factory ReadReply({
    $core.String? data,
  }) {
    final result = create();
    if (data != null) result.data = data;
    return result;
  }

  ReadReply._();

  factory ReadReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReply',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'data')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReply copyWith(void Function(ReadReply) updates) =>
      super.copyWith((message) => updates(message as ReadReply)) as ReadReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReply create() => ReadReply._();
  @$core.override
  ReadReply createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadReply>(create);
  static ReadReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get data => $_getSZ(0);
  @$pb.TagNumber(1)
  set data($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasData() => $_has(0);
  @$pb.TagNumber(1)
  void clearData() => $_clearField(1);
}

/// Message WriteRequest.
class WriteRequest extends $pb.GeneratedMessage {
  factory WriteRequest({
    $core.String? path,
    $core.String? content,
  }) {
    final result = create();
    if (path != null) result.path = path;
    if (content != null) result.content = content;
    return result;
  }

  WriteRequest._();

  factory WriteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory WriteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WriteRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteRequest copyWith(void Function(WriteRequest) updates) =>
      super.copyWith((message) => updates(message as WriteRequest)) as WriteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static WriteRequest create() => WriteRequest._();
  @$core.override
  WriteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static WriteRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteRequest>(create);
  static WriteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);
}

/// Message WriteReply.
class WriteReply extends $pb.GeneratedMessage {
  factory WriteReply({
    $core.int? bytesWritten,
  }) {
    final result = create();
    if (bytesWritten != null) result.bytesWritten = bytesWritten;
    return result;
  }

  WriteReply._();

  factory WriteReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory WriteReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WriteReply',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'bytesWritten', protoName: 'bytes_written')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteReply copyWith(void Function(WriteReply) updates) =>
      super.copyWith((message) => updates(message as WriteReply)) as WriteReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static WriteReply create() => WriteReply._();
  @$core.override
  WriteReply createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static WriteReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteReply>(create);
  static WriteReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get bytesWritten => $_getIZ(0);
  @$pb.TagNumber(1)
  set bytesWritten($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBytesWritten() => $_has(0);
  @$pb.TagNumber(1)
  void clearBytesWritten() => $_clearField(1);
}

/// Message StatRequest.
class StatRequest extends $pb.GeneratedMessage {
  factory StatRequest({
    $core.String? path,
  }) {
    final result = create();
    if (path != null) result.path = path;
    return result;
  }

  StatRequest._();

  factory StatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StatRequest copyWith(void Function(StatRequest) updates) =>
      super.copyWith((message) => updates(message as StatRequest)) as StatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StatRequest create() => StatRequest._();
  @$core.override
  StatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StatRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StatRequest>(create);
  static StatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

/// Message StatReply.
class StatReply extends $pb.GeneratedMessage {
  factory StatReply({
    $core.int? size,
    $core.bool? exists,
  }) {
    final result = create();
    if (size != null) result.size = size;
    if (exists != null) result.exists = exists;
    return result;
  }

  StatReply._();

  factory StatReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StatReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StatReply',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.files'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'size')
    ..aOB(2, _omitFieldNames ? '' : 'exists')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StatReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StatReply copyWith(void Function(StatReply) updates) =>
      super.copyWith((message) => updates(message as StatReply)) as StatReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StatReply create() => StatReply._();
  @$core.override
  StatReply createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StatReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StatReply>(create);
  static StatReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get size => $_getIZ(0);
  @$pb.TagNumber(1)
  set size($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get exists => $_getBF(1);
  @$pb.TagNumber(2)
  set exists($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExists() => $_has(1);
  @$pb.TagNumber(2)
  void clearExists() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
