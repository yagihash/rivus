// The skeleton of a router for the cells of the Dart zoo: a handler is registered with a route, and it is called with
// the request and the segment that the route captures (`/item/<id>`: the text of `id` is the second argument). It is
// real code, which the harness runs and rivus analyses. The call goes through Function.apply, as shelf_router's does:
// which argument is which is not something the analysis follows.
class ZooRequest {
  const ZooRequest(this.path);
  final String path;
}

class ZooRouter {
  final Map<String, Function> _routes = {};

  void get(String route, Function handler) {
    _routes[route] = handler;
  }

  /// The request comes in: the handler of the route is called with the request and the segment of the path.
  Future<Object?> serve(String route, ZooRequest request, String segment) async {
    final handler = _routes[route]!;
    return Function.apply(handler, [request, segment]) as Object?;
  }
}
