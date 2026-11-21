// @cell(F/dyn/func-map)
// @shape: a function value looked up in a map by a constant key is called; the map has another entry with another sink
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/func-map/vuln', vuln);
  zooCase('F/dyn/func-map/fixed', fixed);
  zooCase('F/dyn/func-map/decoy', decoy);
}

void vuln() { // @vuln
  final v = src('dyn-func-map-vuln'); // @source(dyn-func-map-vuln) @miss(unresolved_callee) @ref(FUNC-VALUE-MEMORY,PARITY)
  final m = <String, void Function(String)>{
    'cmd': (x) {
      sinkCmd(x); // @sink(dyn-func-map-vuln)
    },
  };
  m['cmd']!(v);
}

void fixed() { // @fixed
  final v = cleanCmd(src('dyn-func-map-fixed'));
  final m = <String, void Function(String)>{
    'cmd': (x) {
      sinkCmd(x); // @sink(none)
    },
  };
  m['cmd']!(v);
}

void decoy() { // @decoy
  final v = src('dyn-func-map-decoy'); // @source(dyn-func-map-decoy) @miss(unresolved_callee) @ref(FUNC-VALUE-MEMORY,PARITY)
  final m = <String, void Function(String)>{
    'cmd': (x) {
      sinkCmd(x); // @sink(none) @nosink(dyn-func-map-decoy)
    },
    'sql': (x) {
      sinkSql(x); // @sink(dyn-func-map-decoy)
    },
  };
  m['cmd']!('constant');
  m['sql']!(v);
}
