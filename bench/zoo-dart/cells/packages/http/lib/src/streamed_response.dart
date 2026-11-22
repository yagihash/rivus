/// The response of `send` (the shape of `package:http`).
class StreamedResponse {
  StreamedResponse(this.statusCode);
  final int statusCode;
}
