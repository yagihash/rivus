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

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Message FetchRequest.
class FetchRequest extends $pb.GeneratedMessage {
  factory FetchRequest({
    $core.String? url,
  }) {
    final result = create();
    if (url != null) result.url = url;
    return result;
  }

  FetchRequest._();

  factory FetchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FetchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FetchRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.reports'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'url')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FetchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FetchRequest copyWith(void Function(FetchRequest) updates) =>
      super.copyWith((message) => updates(message as FetchRequest)) as FetchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FetchRequest create() => FetchRequest._();
  @$core.override
  FetchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FetchRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<FetchRequest>(create);
  static FetchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get url => $_getSZ(0);
  @$pb.TagNumber(1)
  set url($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUrl() => $_has(0);
  @$pb.TagNumber(1)
  void clearUrl() => $_clearField(1);
}

/// Message FetchReply.
class FetchReply extends $pb.GeneratedMessage {
  factory FetchReply({
    $core.int? status,
    $core.String? body,
  }) {
    final result = create();
    if (status != null) result.status = status;
    if (body != null) result.body = body;
    return result;
  }

  FetchReply._();

  factory FetchReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FetchReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FetchReply',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.reports'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'status')
    ..aOS(2, _omitFieldNames ? '' : 'body')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FetchReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FetchReply copyWith(void Function(FetchReply) updates) =>
      super.copyWith((message) => updates(message as FetchReply)) as FetchReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FetchReply create() => FetchReply._();
  @$core.override
  FetchReply createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FetchReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<FetchReply>(create);
  static FetchReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get status => $_getIZ(0);
  @$pb.TagNumber(1)
  set status($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get body => $_getSZ(1);
  @$pb.TagNumber(2)
  set body($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);
}

/// Message LookupRequest.
class LookupRequest extends $pb.GeneratedMessage {
  factory LookupRequest({
    $core.String? key,
  }) {
    final result = create();
    if (key != null) result.key = key;
    return result;
  }

  LookupRequest._();

  factory LookupRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LookupRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.reports'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'key')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupRequest copyWith(void Function(LookupRequest) updates) =>
      super.copyWith((message) => updates(message as LookupRequest)) as LookupRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LookupRequest create() => LookupRequest._();
  @$core.override
  LookupRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LookupRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LookupRequest>(create);
  static LookupRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get key => $_getSZ(0);
  @$pb.TagNumber(1)
  set key($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);
}

/// Message LookupReply.
class LookupReply extends $pb.GeneratedMessage {
  factory LookupReply({
    $core.String? value,
  }) {
    final result = create();
    if (value != null) result.value = value;
    return result;
  }

  LookupReply._();

  factory LookupReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LookupReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupReply',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'bench.reports'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'value')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupReply copyWith(void Function(LookupReply) updates) =>
      super.copyWith((message) => updates(message as LookupReply)) as LookupReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LookupReply create() => LookupReply._();
  @$core.override
  LookupReply createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LookupReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LookupReply>(create);
  static LookupReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get value => $_getSZ(0);
  @$pb.TagNumber(1)
  set value($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasValue() => $_has(0);
  @$pb.TagNumber(1)
  void clearValue() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
