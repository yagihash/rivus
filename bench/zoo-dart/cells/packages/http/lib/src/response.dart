/// The response of a request (the shape of `package:http`).
class Response {
  Response(this.body, this.statusCode);
  final String body;
  final int statusCode;
}
