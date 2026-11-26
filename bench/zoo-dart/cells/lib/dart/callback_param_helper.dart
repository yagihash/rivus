// @cell(F/dart/callback-param-helper)
// @shape: the route is registered by a helper that is given the function: the function is what the callers of the helper pass, so the parameters of the one that is written at the call of the helper are marked (a function that is not one at the call, such as a variable of two functions, is said by the note in the helper)
import 'package:zoo_mark/zoo_mark.dart';
import 'package:zoo_router/zoo_router.dart';

void register() {
  zooCase('F/dart/callback-param-helper/vuln', vuln);
  zooCase('F/dart/callback-param-helper/fixed', fixed);
  zooCase('F/dart/callback-param-helper/decoy', decoy);
}

void mount(ZooRouter router, Function handler) {
  router.get('/item/<id>', handler);
}

Future<void> vuln() async { // @vuln
  final router = ZooRouter();
  mount(router, (ZooRequest request, String id) async { // @source(dart-callback-param-helper-vuln)
    sinkCmd(id); // @sink(dart-callback-param-helper-vuln)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-vuln'));
}

Future<void> fixed() async { // @fixed
  final router = ZooRouter();
  mount(router, (ZooRequest request, String id) async {
    sinkCmd(cleanCmd(id)); // @sink(none)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-fixed'));
}

Future<void> decoy() async { // @decoy
  final router = ZooRouter();
  mount(router, (ZooRequest request, String id) async { // @source(dart-callback-param-helper-decoy)
    sinkCmd(request.path); // @sink(none) @nosink(dart-callback-param-helper-decoy)
    sinkSql(id); // @sink(dart-callback-param-helper-decoy)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-decoy'));
}
