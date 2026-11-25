// @cell(F/dart/sdk-callback-map)
// @shape: a function that is handed to Map.map and Map.removeWhere is called with the key and the value: what the map holds reaches the sink in the function, and what the function of Map.map returns is what the new map holds (not what the old one held)
// @na(go: a Go map has no member that takes a function; the loop over it is plain code)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/sdk-callback-map/vuln', vuln);
  zooCase('F/dart/sdk-callback-map/fixed', fixed);
  zooCase('F/dart/sdk-callback-map/decoy', decoy);
}

void vuln() { // @vuln
  {
    final t = src('dart-sdk-callback-map-vuln-key'); // @source(dart-sdk-callback-map-vuln-key)
    final m = <String, String>{t: 'v'};
    m.map((k, v) {
      sinkCmd(k); // @sink(dart-sdk-callback-map-vuln-key)
      return MapEntry(k, v);
    });
  }
  {
    final t = src('dart-sdk-callback-map-vuln-value'); // @source(dart-sdk-callback-map-vuln-value)
    final m = <String, String>{'k': t};
    m.map((k, v) {
      sinkCmd(v); // @sink(dart-sdk-callback-map-vuln-value)
      return MapEntry(k, v);
    });
  }
  {
    final t = src('dart-sdk-callback-map-vuln-result'); // @source(dart-sdk-callback-map-vuln-result)
    final m = <String, String>{'k': t};
    final n = m.map((k, v) => MapEntry(k, v));
    sinkCmd(n['k']!); // @sink(dart-sdk-callback-map-vuln-result)
  }
  {
    final t = src('dart-sdk-callback-map-vuln-removewhere'); // @source(dart-sdk-callback-map-vuln-removewhere)
    final m = <String, String>{'k': t};
    m.removeWhere((k, v) {
      sinkCmd(v); // @sink(dart-sdk-callback-map-vuln-removewhere)
      return false;
    });
  }
}

void fixed() { // @fixed
  {
    final m = <String, String>{'k': cleanCmd(src('dart-sdk-callback-map-fixed'))};
    m.map((k, v) {
      sinkCmd(v); // @sink(none)
      return MapEntry(k, v);
    });
    final n = m.map((k, v) => MapEntry(k, v));
    sinkCmd(n['k']!); // @sink(none)
    m.removeWhere((k, v) {
      sinkCmd(v); // @sink(none)
      return false;
    });
  }
}

void decoy() { // @decoy
  {
    final t = src('dart-sdk-callback-map-decoy'); // @source(dart-sdk-callback-map-decoy)
    final m = <String, String>{'k': 'constant'};
    m.map((k, v) {
      sinkCmd(v); // @sink(none) @nosink(dart-sdk-callback-map-decoy)
      return MapEntry(k, v);
    });
    m.removeWhere((k, v) {
      sinkCmd(v); // @sink(none) @nosink(dart-sdk-callback-map-decoy)
      return false;
    });
    sinkSql(t); // @sink(dart-sdk-callback-map-decoy)
  }
  // the map is made of what the function returns: the old map is not in it
  {
    final t = src('dart-sdk-callback-map-decoy-result'); // @source(dart-sdk-callback-map-decoy-result)
    final m = <String, String>{'k': t};
    final n = m.map((k, v) => MapEntry('k', 'constant'));
    sinkCmd(n['k']!); // @sink(none) @nosink(dart-sdk-callback-map-decoy-result)
    sinkSql(t); // @sink(dart-sdk-callback-map-decoy-result)
  }
}
