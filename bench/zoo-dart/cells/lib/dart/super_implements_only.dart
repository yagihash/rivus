// @cell(F/dart/super-implements-only)
// @na(go: no mixins; the nearest is embedding (dyn/embed))
// @shape: the class implements another class (which has the body) and an interface, and writes its own method; `implements` takes no body, so a call through the interface does not reach the body of the implemented class
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-implements-only/vuln', vuln);
  zooCase('F/dart/super-implements-only/fixed', fixed);
  zooCase('F/dart/super-implements-only/decoy', decoy);
}

abstract class Store {
  void put(String s);
}

abstract class StoreFixed {
  void put(String s);
}

abstract class StoreDecoy {
  void put(String s);
}

class HasBody {
  void put(String s) {
    sinkCmd(s); // @sink(none) @nosink(dart-super-implements-only-decoy)
  }
}

class OnlyImplements implements HasBody, Store {
  @override
  void put(String s) {
    sinkSql(s); // @sink(dart-super-implements-only-vuln)
  }
}

void vuln() { // @vuln
  final Store store = OnlyImplements();
  store.put(src('dart-super-implements-only-vuln')); // @source(dart-super-implements-only-vuln)
}

class OnlyImplementsFixed implements HasBody, StoreFixed {
  @override
  void put(String s) {
    sinkSql(s); // @sink(none)
  }
}

void fixed() { // @fixed
  final StoreFixed store = OnlyImplementsFixed();
  store.put(cleanSql(src('dart-super-implements-only-fixed')));
}

class OnlyImplementsDecoy implements HasBody, StoreDecoy {
  @override
  void put(String s) {
    sinkHtml(s); // @sink(dart-super-implements-only-decoy)
  }
}

void decoy() { // @decoy
  HasBody().put('constant');
  final StoreDecoy store = OnlyImplementsDecoy();
  store.put(src('dart-super-implements-only-decoy')); // @source(dart-super-implements-only-decoy)
}
