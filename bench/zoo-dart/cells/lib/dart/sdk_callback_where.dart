// @cell(F/dart/sdk-callback-where)
// @shape: a function that is handed to a function of the core library that tests the elements of the receiver (removeWhere, retainWhere, indexWhere, takeWhile and the like) is called with an element: what the list holds reaches the sink in the function. A function that uses a value that is not an element is not a finding
// @na(go: a Go function that is handed to the standard library has its cells in F/clos and the callbacks of the standard library; Go has no member that tests the elements of a collection)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/sdk-callback-where/vuln', vuln);
  zooCase('F/dart/sdk-callback-where/fixed', fixed);
  zooCase('F/dart/sdk-callback-where/decoy', decoy);
}

void vuln() { // @vuln
  {
    final t = src('dart-sdk-callback-where-vuln-removewhere'); // @source(dart-sdk-callback-where-vuln-removewhere)
    final l = <String>[t];
    l.removeWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-removewhere)
      return false;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-retainwhere'); // @source(dart-sdk-callback-where-vuln-retainwhere)
    final l = <String>[t];
    l.retainWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-retainwhere)
      return true;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-setremovewhere'); // @source(dart-sdk-callback-where-vuln-setremovewhere)
    final s = <String>{t};
    s.removeWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-setremovewhere)
      return false;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-setretainwhere'); // @source(dart-sdk-callback-where-vuln-setretainwhere)
    final s = <String>{t};
    s.retainWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-setretainwhere)
      return true;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-indexwhere'); // @source(dart-sdk-callback-where-vuln-indexwhere)
    final l = <String>[t];
    l.indexWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-indexwhere)
      return true;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-lastindexwhere'); // @source(dart-sdk-callback-where-vuln-lastindexwhere)
    final l = <String>[t];
    l.lastIndexWhere((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-lastindexwhere)
      return true;
    });
  }
  {
    final t = src('dart-sdk-callback-where-vuln-takewhile'); // @source(dart-sdk-callback-where-vuln-takewhile)
    final l = <String>[t];
    l.takeWhile((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-takewhile)
      return true;
    }).toList();
  }
  {
    final t = src('dart-sdk-callback-where-vuln-skipwhile'); // @source(dart-sdk-callback-where-vuln-skipwhile)
    final l = <String>[t];
    l.skipWhile((e) {
      sinkCmd(e); // @sink(dart-sdk-callback-where-vuln-skipwhile)
      return false;
    }).toList();
  }
}

void fixed() { // @fixed
  {
    final l = <String>[cleanCmd(src('dart-sdk-callback-where-fixed'))];
    l.removeWhere((e) {
      sinkCmd(e); // @sink(none)
      return false;
    });
    l.retainWhere((e) {
      sinkCmd(e); // @sink(none)
      return true;
    });
    l.indexWhere((e) {
      sinkCmd(e); // @sink(none)
      return true;
    });
    l.lastIndexWhere((e) {
      sinkCmd(e); // @sink(none)
      return true;
    });
    l.takeWhile((e) {
      sinkCmd(e); // @sink(none)
      return true;
    }).toList();
    l.skipWhile((e) {
      sinkCmd(e); // @sink(none)
      return false;
    }).toList();
  }
  {
    final s = <String>{cleanCmd(src('dart-sdk-callback-where-fixed-set'))};
    s.removeWhere((e) {
      sinkCmd(e); // @sink(none)
      return false;
    });
    s.retainWhere((e) {
      sinkCmd(e); // @sink(none)
      return true;
    });
  }
}

void decoy() { // @decoy
  // the elements are constants; the value of the source is in a variable that the function does not use as its element
  {
    final t = src('dart-sdk-callback-where-decoy'); // @source(dart-sdk-callback-where-decoy)
    final l = <String>['constant'];
    l.removeWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return false;
    });
    l.retainWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return true;
    });
    l.indexWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return true;
    });
    l.lastIndexWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return true;
    });
    l.takeWhile((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return true;
    }).toList();
    l.skipWhile((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return false;
    }).toList();
    final s = <String>{'constant'};
    s.removeWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return false;
    });
    s.retainWhere((e) {
      sinkCmd(e); // @sink(none) @nosink(dart-sdk-callback-where-decoy)
      return true;
    });
    sinkSql(t); // @sink(dart-sdk-callback-where-decoy)
  }
}
