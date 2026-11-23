// @cell(F/dart/callback-param-closure)
// @shape: a function written where the route is registered is called by the router with the segment of the path as its second argument: the segment is user input (callback_param of the rules), the request is not what is marked
import 'package:zoo_mark/zoo_mark.dart';
import 'package:zoo_router/zoo_router.dart';

void register() {
  zooCase('F/dart/callback-param-closure/vuln', vuln);
  zooCase('F/dart/callback-param-closure/fixed', fixed);
  zooCase('F/dart/callback-param-closure/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final router = ZooRouter();
  router.get('/item/<id>', (ZooRequest request, String id) async { // @source(dart-callback-param-closure-vuln)
    sinkCmd(id); // @sink(dart-callback-param-closure-vuln)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-closure-vuln'));
}

Future<void> fixed() async { // @fixed
  final router = ZooRouter();
  router.get('/item/<id>', (ZooRequest request, String id) async {
    sinkCmd(cleanAll(id)); // @sink(none)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-closure-fixed'));
}

Future<void> decoy() async { // @decoy
  final router = ZooRouter();
  router.get('/item/<id>', (ZooRequest request, String id) async { // @source(dart-callback-param-closure-decoy)
    sinkCmd(request.path); // @sink(none) @nosink(dart-callback-param-closure-decoy)
    sinkSql(id); // @sink(dart-callback-param-closure-decoy)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-closure-decoy'));
}
