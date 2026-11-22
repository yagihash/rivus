// The skeleton of `Dio` of package:dio: an abstract class whose methods take the URL (or the path, which dio puts after
// `baseUrl`; an absolute one replaces it) first. It is real code, not a declaration: the harness runs it and it records
// the URL that a request would go to, and the place that `download` writes to, as what reached the sink; rivus does not
// read it (the built-in rows of package:dio stand for the body of a dependency's method).
import 'package:zoo_mark/zoo_mark.dart';

import 'response.dart';

abstract class Dio {
  factory Dio() => _SkeletonDio();

  Future<Response<T>> get<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
  });

  Future<Response<T>> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
  });

  Future<Response<T>> getUri<T>(Uri uri, {Object? data});

  Future<Response> download(
    String urlPath,
    dynamic savePath, {
    Object? data,
    Map<String, dynamic>? queryParameters,
  });
}

class _SkeletonDio implements Dio {
  @override
  Future<Response<T>> get<T>(String path, {Object? data, Map<String, dynamic>? queryParameters}) async {
    recordAny(Uri.decodeFull(path));
    return Response<T>(statusCode: 200);
  }

  @override
  Future<Response<T>> post<T>(String path, {Object? data, Map<String, dynamic>? queryParameters}) async {
    recordAny(Uri.decodeFull(path));
    return Response<T>(statusCode: 200);
  }

  @override
  Future<Response<T>> getUri<T>(Uri uri, {Object? data}) async {
    recordAny(Uri.decodeFull(uri.toString()));
    return Response<T>(statusCode: 200);
  }

  @override
  Future<Response> download(String urlPath, dynamic savePath, {Object? data, Map<String, dynamic>? queryParameters}) async {
    recordAny(Uri.decodeFull(urlPath));
    if (savePath is String) recordAny(savePath);
    return Response(statusCode: 200);
  }
}
