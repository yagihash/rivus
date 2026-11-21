// @cell(F/dyn/sink-behind-own-interface)
// @shape: the project calls its own abstract class (a Querier, a Repository), the call site is the abstract method, and the sink is in an implementation
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/sink-behind-own-interface/vuln', vuln);
  zooCase('F/dyn/sink-behind-own-interface/fixed', fixed);
  zooCase('F/dyn/sink-behind-own-interface/decoy', decoy);
}

abstract class Querier {
  void query(String sql);
}

class PgQuerier implements Querier {
  @override
  void query(String sql) {
    sinkSql(sql); // @sink(dyn-sink-behind-own-interface-vuln)
  }
}

void vuln() { // @vuln
  final Querier q = PgQuerier();
  q.query(src('dyn-sink-behind-own-interface-vuln')); // @source(dyn-sink-behind-own-interface-vuln)
}

abstract class QuerierFixed {
  void query(String sql);
}

class PgQuerierFixed implements QuerierFixed {
  @override
  void query(String sql) {
    sinkSql(sql); // @sink(none)
  }
}

void fixed() { // @fixed
  final QuerierFixed q = PgQuerierFixed();
  q.query(cleanSql(src('dyn-sink-behind-own-interface-fixed')));
}

// the only implementation has no sink for what it is given
abstract class QuerierQuiet {
  void query(String sql);
}

class QuietQuerier implements QuerierQuiet {
  @override
  void query(String sql) {
    sinkSql('select 1'); // @sink(none) @nosink(dyn-sink-behind-own-interface-decoy)
  }
}

void decoy() { // @decoy
  final QuerierQuiet q = QuietQuerier();
  final v = src('dyn-sink-behind-own-interface-decoy'); // @source(dyn-sink-behind-own-interface-decoy)
  q.query(v);
  sinkCmd(v); // @sink(dyn-sink-behind-own-interface-decoy)
}
