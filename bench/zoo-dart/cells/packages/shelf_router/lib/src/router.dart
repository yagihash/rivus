// The skeleton of `Router` of package:shelf_router: real code (the harness runs it), not read by rivus (the built-in
// rows of package:shelf_router stand for it). The route `/x/<id>` captures the segment `id`, as the real one does.
import 'package:shelf/shelf.dart';

class Router {
  final List<(String, Function)> _routes = [];

  void get(String route, Function handler) => _routes.add((route, handler));

  Future<Response> call(Request request) async {
    for (final (route, handler) in _routes) {
      final names = RegExp('<([^>]+)>').allMatches(route).map((m) => m[1]!).toList();
      final re = RegExp('^${route.replaceAll(RegExp('<[^>]+>'), '(.*)')}\$');
      final m = re.firstMatch(Uri.decodeComponent(request.requestedUri.path));
      if (m == null) continue;
      final params = <String, String>{for (var i = 0; i < names.length; i++) names[i]: m[i + 1]!};
      final req = request.change(context: {'shelf_router/params': params});
      if (handler is Handler || names.isEmpty) return await handler(req) as Response;
      return await Function.apply(handler, [req, ...names.map((n) => params[n])]) as Response;
    }
    return Response.ok('');
  }
}

extension RouterParams on Request {
  Map<String, String> get params => (context['shelf_router/params'] as Map<String, String>?) ?? const {};
}
