class Request {
  final String method;
  final Uri requestedUri;
  final Map<String, Object?> context;

  Request(this.method, this.requestedUri, {Map<String, Object?>? context}) : context = context ?? const {};

  Request change({Map<String, Object?>? context}) =>
      Request(method, requestedUri, context: {...this.context, ...?context});
}
