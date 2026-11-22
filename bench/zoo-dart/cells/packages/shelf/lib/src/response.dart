// A skeleton of package:shelf, made from its public signatures (shelf 1.4). The harness does not run it: exec/pubspec.yaml
// overrides `shelf` with the real package. The body only keeps what it was given (a field), as the real one does.
import 'dart:convert';

class Response {
  final int statusCode;
  final Object? _body;
  final Map<String, String> headers;

  // the public constructors go through this one: a call to `Response.new` inside the skeleton would be a site of the row of it
  Response._(this.statusCode, Object? body, Map<String, Object>? headers)
      : _body = body,
        headers = {for (final e in (headers ?? const <String, Object>{}).entries) e.key.toLowerCase(): '${e.value}'};

  Response(
    int statusCode, {
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(statusCode, body, headers);

  Response.ok(
    Object? body, {
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(200, body, headers);

  Response.movedPermanently(
    Object location, {
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(301, body, {...?headers, 'location': '$location'});

  Response.found(
    Object location, {
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(302, body, {...?headers, 'location': '$location'});

  Response.seeOther(
    Object location, {
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(303, body, {...?headers, 'location': '$location'});

  Response.badRequest({
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(400, body, headers);

  Response.unauthorized(
    Object? body, {
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(401, body, headers);

  Response.forbidden(
    Object? body, {
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(403, body, headers);

  Response.notFound(
    Object? body, {
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(404, body, headers);

  Response.internalServerError({
    Object? body,
    Map<String, Object>? headers,
    Encoding? encoding,
    Map<String, Object>? context,
  }) : this._(500, body, headers);

  Future<String> readAsString([Encoding? encoding]) async => '${_body ?? ''}';

  Response change({Map<String, Object?>? headers, Map<String, Object?>? context, Object? body}) =>
      Response._(statusCode, body ?? _body, null);
}
