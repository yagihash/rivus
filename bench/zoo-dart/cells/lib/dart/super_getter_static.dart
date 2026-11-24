// @cell(F/dart/super-getter-static)
// @na(go: no getters or super)
// @shape: super.x runs the getter of the direct superclass and not the getter of the subclass that is calling (the decoy reads super.x of a tainted object, and x of a clean one)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/super-getter-static/vuln', vuln);
  zooCase('F/dart/super-getter-static/fixed', fixed);
  zooCase('F/dart/super-getter-static/decoy', decoy);
}

class Base {
  String data = '';

  String get x {
    sinkCmd(data); // @sink(dart-super-getter-static-vuln)
    return '';
  }
}

class Sub extends Base {
  @override
  String get x {
    return super.x;
  }

  String readSuper() => super.x;
}

void vuln() { // @vuln
  final a = Sub();
  a.data = src('dart-super-getter-static-vuln'); // @source(dart-super-getter-static-vuln)
  a.x.length;
  a.readSuper().length;
}

class BaseFixed {
  String data = '';

  String get x {
    sinkCmd(data); // @sink(none)
    return '';
  }
}

class SubFixed extends BaseFixed {
  @override
  String get x {
    return super.x;
  }

  String readSuper() => super.x;
}

void fixed() { // @fixed
  final a = SubFixed();
  a.data = cleanCmd(src('dart-super-getter-static-fixed'));
  a.x.length;
  a.readSuper().length;
}

class BaseDecoy {
  String data = '';

  String get x {
    return '';
  }
}

class SubDecoy extends BaseDecoy {
  @override
  String get x {
    sinkCmd(data); // @sink(none) @nosink(dart-super-getter-static-decoy)
    return '';
  }

  String readSuper() => super.x;
}

void decoy() { // @decoy
  final tainted = SubDecoy();
  tainted.data = src('dart-super-getter-static-decoy'); // @source(dart-super-getter-static-decoy)
  tainted.readSuper().length;
  final clean = SubDecoy();
  clean.data = 'constant';
  clean.x.length;
  sinkSql(tainted.data); // @sink(dart-super-getter-static-decoy)
}
