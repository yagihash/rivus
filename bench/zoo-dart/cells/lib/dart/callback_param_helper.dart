// @cell(F/dart/callback-param-helper)
// @shape: the route is registered by a helper that is given the function: which function the router is given is not known at the call that the rule names (a parameter of the helper), so the parameters are not marked; the note (unresolved_callee) is in the helper, which is neither the function of the source nor the one of the sink
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
  mount(router, (ZooRequest request, String id) async { // @source(dart-callback-param-helper-vuln) @miss(silent) @ref(#149)
    sinkCmd(id); // @sink(dart-callback-param-helper-vuln)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-vuln'));
}

Future<void> fixed() async { // @fixed
  final router = ZooRouter();
  mount(router, (ZooRequest request, String id) async {
    sinkCmd(cleanAll(id)); // @sink(none)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-fixed'));
}

Future<void> decoy() async { // @decoy
  final router = ZooRouter();
  mount(router, (ZooRequest request, String id) async { // @source(dart-callback-param-helper-decoy) @miss(silent) @ref(#149)
    sinkCmd(request.path); // @sink(none) @nosink(dart-callback-param-helper-decoy)
    sinkSql(id); // @sink(dart-callback-param-helper-decoy)
    return null;
  });
  await router.serve('/item/<id>', const ZooRequest('/item/x'), typed('dart-callback-param-helper-decoy'));
}
