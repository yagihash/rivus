// @cell(F/dart/flutter-route-args)
// @na(go: no such form; the arguments that a caller gave to a route)
// @shape: the source is the arguments of RouteSettings, which the Navigator hands to the builder of the route that another screen pushed
import 'package:zoo_flutter/zoo_flutter.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/flutter-route-args/vuln', vuln);
  zooCase('F/dart/flutter-route-args/fixed', fixed);
  zooCase('F/dart/flutter-route-args/decoy', decoy);
}

void vuln() { // @vuln
  final nav = Navigator();
  nav.routes['/detail'] = (settings) {
    final id = settings.arguments as String; // @source(dart-flutter-route-args-vuln)
    sinkPath('/srv/base', id); // @sink(dart-flutter-route-args-vuln)
  };
  nav.pushNamed('/detail', arguments: typed('dart-flutter-route-args-vuln'));
}

void fixed() { // @fixed
  final nav = Navigator();
  nav.routes['/detail'] = (settings) {
    final id = cleanPath(settings.arguments as String);
    sinkPath('/srv/base', id); // @sink(none)
  };
  nav.pushNamed('/detail', arguments: typed('dart-flutter-route-args-fixed'));
}

void decoy() { // @decoy
  final nav = Navigator();
  nav.routes['/detail'] = (settings) {
    final id = settings.arguments as String; // @source(dart-flutter-route-args-decoy)
    sinkPath('/srv/base', 'constant'); // @sink(none) @nosink(dart-flutter-route-args-decoy)
    sinkSql(id); // @sink(dart-flutter-route-args-decoy)
  };
  nav.pushNamed('/detail', arguments: typed('dart-flutter-route-args-decoy'));
}
