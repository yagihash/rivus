// @cell(F/dart/shelf-router-extra-param)
// @elsewhere(go: F/pkg/gin/query)
// @shape: the handler of Router.get takes the captured segment as a String parameter after the Request (shelf_router passes it): the text is user input, and the built-in callback_param rows of shelf_router say so
import 'package:shelf/shelf.dart';
import 'package:shelf_router/shelf_router.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/shelf-router-extra-param/vuln', vuln);
  zooCase('F/dart/shelf-router-extra-param/fixed', fixed);
  zooCase('F/dart/shelf-router-extra-param/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final router = Router();
  router.get('/item/<id>', (Request request, String id) async { // @source(dart-shelf-router-extra-param-vuln)
    sinkCmd(id); // @sink(dart-shelf-router-extra-param-vuln)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-extra-param-vuln')))));
}

Future<void> fixed() async { // @fixed
  final router = Router();
  router.get('/item/<id>', (Request request, String id) async {
    sinkCmd(cleanCmd(id)); // @sink(none)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-extra-param-fixed')))));
}

Future<void> decoy() async { // @decoy
  final router = Router();
  router.get('/item/<id>', (Request request, String id) async { // @source(dart-shelf-router-extra-param-decoy)
    sinkCmd('constant'); // @sink(none) @nosink(dart-shelf-router-extra-param-decoy)
    sinkSql(id); // @sink(dart-shelf-router-extra-param-decoy)
    return Response.ok('');
  });
  await router(Request('GET', Uri.parse('http://localhost/item/' + Uri.encodeComponent(typed('dart-shelf-router-extra-param-decoy')))));
}
