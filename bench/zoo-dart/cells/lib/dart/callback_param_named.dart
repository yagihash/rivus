// @cell(F/dart/callback-param-named)
// @shape: a function that has a name (and a variable that holds a function written once) is given to the router in place of a function literal: it is found as well, and its parameter after the request is user input; the other two arguments of the same name are not
import 'package:zoo_mark/zoo_mark.dart';
import 'package:zoo_router/zoo_router.dart';

void register() {
  zooCase('F/dart/callback-param-named/vuln', vuln);
  zooCase('F/dart/callback-param-named/fixed', fixed);
  zooCase('F/dart/callback-param-named/decoy', decoy);
}

Future<Object?> showItem(ZooRequest request, String id) async { // @source(dart-callback-param-named-vuln)
  sinkCmd(id); // @sink(dart-callback-param-named-vuln)
  return null;
}

Future<void> vuln() async { // @vuln
  final router = ZooRouter();
  router.get('/item/<id>', showItem);
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-named-vuln'));
}

Future<Object?> showItemFixed(ZooRequest request, String id) async {
  sinkCmd(cleanCmd(id)); // @sink(none)
  return null;
}

Future<void> fixed() async { // @fixed
  final router = ZooRouter();
  router.get('/item/<id>', showItemFixed);
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-named-fixed'));
}

Future<void> decoy() async { // @decoy
  final router = ZooRouter();
  final handler = (ZooRequest request, String id) async { // @source(dart-callback-param-named-decoy)
    sinkCmd(request.path); // @sink(none) @nosink(dart-callback-param-named-decoy)
    sinkSql(id); // @sink(dart-callback-param-named-decoy)
    return null;
  };
  router.get('/item/<id>', handler);
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-named-decoy'));
}
