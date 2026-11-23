// @cell(F/dart/super-mixin-impl)
// @na(go: no mixins; the nearest is embedding (dyn/embed))
// @shape: the method that is called is implemented by a mixin (or by a superclass) of the class, and the variable's type is only the interface that the class implements (the decoy's mixin sinks the constant and the value goes to another sink)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-mixin-impl/vuln', vuln);
  zooCase('F/dart/super-mixin-impl/vuln2', vuln2);
  zooCase('F/dart/super-mixin-impl/fixed', fixed);
  zooCase('F/dart/super-mixin-impl/decoy', decoy);
}

abstract class Store {
  void put(String s);
}

mixin Saver {
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-vuln)
  }
}

class DiskStore extends Object with Saver implements Store {}

void vuln() { // @vuln
  final Store store = DiskStore();
  store.put(src('dart-super-mixin-impl-vuln')); // @source(dart-super-mixin-impl-vuln) @miss(dynamic_dispatch,unmodeled_library_call) @ref(R-16)
}

class BaseSaver {
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-vuln2)
  }
}

class SuperStore extends BaseSaver implements Store {}

void vuln2() { // @vuln
  final Store store = SuperStore();
  store.put(src('dart-super-mixin-impl-vuln2')); // @source(dart-super-mixin-impl-vuln2) @miss(dynamic_dispatch,unmodeled_library_call) @ref(R-16)
}

abstract class StoreFixed {
  void put(String s);
}

mixin SaverFixed {
  void put(String s) {
    sinkCmd(s); // @sink(none)
  }
}

class DiskStoreFixed extends Object with SaverFixed implements StoreFixed {}

void fixed() { // @fixed
  final StoreFixed store = DiskStoreFixed();
  store.put(cleanCmd(src('dart-super-mixin-impl-fixed')));
}

abstract class StoreDecoy {
  void put(String s);
}

mixin SaverDecoy {
  void put(String s) {
    sinkCmd('constant'); // @sink(none) @nosink(dart-super-mixin-impl-decoy)
    sinkSql(s); // @sink(dart-super-mixin-impl-decoy)
  }
}

class DiskStoreDecoy extends Object with SaverDecoy implements StoreDecoy {}

void decoy() { // @decoy
  final StoreDecoy store = DiskStoreDecoy();
  store.put(src('dart-super-mixin-impl-decoy')); // @source(dart-super-mixin-impl-decoy) @miss(dynamic_dispatch,unmodeled_library_call) @ref(R-16)
}
