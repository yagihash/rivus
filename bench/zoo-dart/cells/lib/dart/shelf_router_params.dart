// @cell(F/dart/shelf-router-params)
// @shape: a handler registered with Router.get of package:shelf_router (a skeleton of it) reads a path parameter with Request.params: the Request is the source of the built-in rows
import 'package:shelf/shelf.dart';
import 'package:shelf_router/shelf_router.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/shelf-router-params/vuln', vuln);
  zooCase('F/dart/shelf-router-params/fixed', fixed);
  zooCase('F/dart/shelf-router-params/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final router = Router();
  router.get('/item/<id>', (Request request) async { // @source(dart-shelf-router-params-vuln)
    sinkCmd(request.params['id']!); // @sink(dart-shelf-router-params-vuln)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-params-vuln')))));
}

Future<void> fixed() async { // @fixed
  final router = Router();
  router.get('/item/<id>', (Request request) async {
    sinkCmd(cleanCmd(request.params['id']!)); // @sink(none)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-params-fixed')))));
}

Future<void> decoy() async { // @decoy
  final router = Router();
  router.get('/item/<id>', (Request request) async { // @source(dart-shelf-router-params-decoy)
    sinkCmd('constant'); // @sink(none) @nosink(dart-shelf-router-params-decoy)
    sinkSql(request.params['id']!); // @sink(dart-shelf-router-params-decoy)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-params-decoy')))));
}
