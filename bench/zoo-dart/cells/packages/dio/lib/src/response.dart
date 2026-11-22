/// The response of a request (the shape of `package:dio`: `data` is what the server sent).
class Response<T> {
  Response({this.data, this.statusCode});
  T? data;
  int? statusCode;
}
