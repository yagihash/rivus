// @cell(F/dart/super-mixin-override)
// @na(go: no mixins; the nearest is embedding (dyn/embed))
// @shape: the class takes a method from a mixin and writes its own with the same name; a call through the interface reaches the one that the class writes, and the mixin's body is not reached
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-mixin-override/vuln', vuln);
  zooCase('F/dart/super-mixin-override/fixed', fixed);
  zooCase('F/dart/super-mixin-override/decoy', decoy);
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

mixin Saver {
  void put(String s) {
    sinkCmd(s); // @sink(none) @nosink(dart-super-mixin-override-decoy)
  }
}

class SaverUser extends Object with Saver {}

class DiskStore extends Object with Saver implements Store {
  @override
  void put(String s) {
    sinkSql(s); // @sink(dart-super-mixin-override-vuln)
  }
}

void vuln() { // @vuln
  final Store store = DiskStore();
  store.put(src('dart-super-mixin-override-vuln')); // @source(dart-super-mixin-override-vuln)
}

class DiskStoreFixed extends Object with Saver implements StoreFixed {
  @override
  void put(String s) {
    sinkSql(s); // @sink(none)
  }
}

void fixed() { // @fixed
  final StoreFixed store = DiskStoreFixed();
  store.put(cleanSql(src('dart-super-mixin-override-fixed')));
}

class DiskStoreDecoy extends Object with Saver implements StoreDecoy {
  @override
  void put(String s) {
    sinkHtml(s); // @sink(dart-super-mixin-override-decoy)
  }
}

void decoy() { // @decoy
  SaverUser().put('constant');
  final StoreDecoy store = DiskStoreDecoy();
  store.put(src('dart-super-mixin-override-decoy')); // @source(dart-super-mixin-override-decoy)
}
