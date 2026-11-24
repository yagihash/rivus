// @cell(F/dart/super-mixin-impl)
// @na(go: no mixins; the nearest is embedding (dyn/embed))
// @shape: the method that is called is implemented by a mixin (or by a superclass) of the class, and the variable's type is only the interface that the class implements (the decoy's mixin sinks the constant and the value goes to another sink)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-mixin-impl/vuln', vuln);
  zooCase('F/dart/super-mixin-impl/vuln2', vuln2);
  zooCase('F/dart/super-mixin-impl/fixed', fixed);
  zooCase('F/dart/super-mixin-impl/decoy', decoy);
  zooCase('F/dart/super-mixin-impl/vuln3', withOrder);
  zooCase('F/dart/super-mixin-impl/vuln4', onConstraint);
  zooCase('F/dart/super-mixin-impl/vuln5', deepSuper);
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
  store.put(src('dart-super-mixin-impl-vuln')); // @source(dart-super-mixin-impl-vuln)
}

class BaseSaver {
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-vuln2)
  }
}

abstract class StoreSuper {
  void put(String s);
}

class SuperStore extends BaseSaver implements StoreSuper {}

void vuln2() { // @vuln
  final StoreSuper store = SuperStore();
  store.put(src('dart-super-mixin-impl-vuln2')); // @source(dart-super-mixin-impl-vuln2)
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
  store.put(src('dart-super-mixin-impl-decoy')); // @source(dart-super-mixin-impl-decoy)
}

// `with A, B`: the last mixin wins, and the one before it is not reached.
abstract class StoreOrder {
  void put(String s);
}

mixin FirstSaver {
  void put(String s) {
    sinkSql(s); // @sink(none) @nosink(dart-super-mixin-impl-with-order)
  }
}

mixin LastSaver {
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-with-order)
  }
}

class FirstUser extends Object with FirstSaver {}

class OrderStore extends Object with FirstSaver, LastSaver implements StoreOrder {}

void withOrder() { // @vuln
  FirstUser().put('constant');
  final StoreOrder store = OrderStore();
  store.put(src('dart-super-mixin-impl-with-order')); // @source(dart-super-mixin-impl-with-order)
}

// A mixin with an `on` constraint wins over the superclass that it is on.
abstract class StoreOn {
  void put(String s);
}

class BaseOn {
  void put(String s) {
    sinkSql(s); // @sink(none) @nosink(dart-super-mixin-impl-on-constraint)
  }
}

mixin SaverOn on BaseOn {
  @override
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-on-constraint)
  }
}

class OnStore extends BaseOn with SaverOn implements StoreOn {}

void onConstraint() { // @vuln
  BaseOn().put('constant');
  final StoreOn store = OnStore();
  store.put(src('dart-super-mixin-impl-on-constraint')); // @source(dart-super-mixin-impl-on-constraint)
}

// The implementation is two classes up.
abstract class StoreDeep {
  void put(String s);
}

class RootDeep {
  void put(String s) {
    sinkCmd(s); // @sink(dart-super-mixin-impl-deep-super)
  }
}

class MiddleDeep extends RootDeep {}

class LeafDeep extends MiddleDeep implements StoreDeep {}

void deepSuper() { // @vuln
  final StoreDeep store = LeafDeep();
  store.put(src('dart-super-mixin-impl-deep-super')); // @source(dart-super-mixin-impl-deep-super)
}
