class Response {
  final int statusCode;

  Response(this.statusCode, {Object? body});

  Response.ok(Object? body) : this(200, body: body);
}
