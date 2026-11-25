// @cell(F/dart/collection-where-or-null)
// @shape: a function that is handed to firstWhereOrNull, lastWhereOrNull, singleWhereOrNull, whereNot and none of package:collection (a skeleton of it) is called with an element: what the list holds reaches the sink in the function. A function that uses a value that is not an element is not a finding
// @na(go: a Go function that is handed to a library function has its cells in F/clos; the list helpers of a package are Dart's)
import 'package:collection/collection.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/collection-where-or-null/vuln', vuln);
  zooCase('F/dart/collection-where-or-null/fixed', fixed);
  zooCase('F/dart/collection-where-or-null/decoy', decoy);
}

void vuln() { // @vuln
  {
    final t = src('dart-collection-where-or-null-vuln-first'); // @source(dart-collection-where-or-null-vuln-first)
    final l = <String>[t];
    l.firstWhereOrNull((e) {
      sinkCmd(e); // @sink(dart-collection-where-or-null-vuln-first)
      return true;
    });
  }
  {
    final t = src('dart-collection-where-or-null-vuln-last'); // @source(dart-collection-where-or-null-vuln-last)
    final l = <String>[t];
    l.lastWhereOrNull((e) {
      sinkCmd(e); // @sink(dart-collection-where-or-null-vuln-last)
      return true;
    });
  }
  {
    final t = src('dart-collection-where-or-null-vuln-single'); // @source(dart-collection-where-or-null-vuln-single)
    final l = <String>[t];
    l.singleWhereOrNull((e) {
      sinkCmd(e); // @sink(dart-collection-where-or-null-vuln-single)
      return true;
    });
  }
  {
    final t = src('dart-collection-where-or-null-vuln-wherenot'); // @source(dart-collection-where-or-null-vuln-wherenot)
    final l = <String>[t];
    l.whereNot((e) {
      sinkCmd(e); // @sink(dart-collection-where-or-null-vuln-wherenot)
      return false;
    }).toList();
  }
  {
    final t = src('dart-collection-where-or-null-vuln-none'); // @source(dart-collection-where-or-null-vuln-none)
    final l = <String>[t];
    l.none((e) {
      sinkCmd(e); // @sink(dart-collection-where-or-null-vuln-none)
      return false;
    });
  }
  {
    final t = src('dart-collection-where-or-null-vuln-result'); // @source(dart-collection-where-or-null-vuln-result)
    final l = <String>[t];
    final found = l.firstWhereOrNull((e) => true);
    sinkCmd(found!); // @sink(dart-collection-where-or-null-vuln-result)
  }
}

void fixed() { // @fixed
  final l = <String>[cleanCmd(src('dart-collection-where-or-null-fixed'))];
  l.firstWhereOrNull((e) {
    sinkCmd(e); // @sink(none)
    return true;
  });
  l.lastWhereOrNull((e) {
    sinkCmd(e); // @sink(none)
    return true;
  });
  l.singleWhereOrNull((e) {
    sinkCmd(e); // @sink(none)
    return true;
  });
  l.whereNot((e) {
    sinkCmd(e); // @sink(none)
    return false;
  }).toList();
  l.none((e) {
    sinkCmd(e); // @sink(none)
    return false;
  });
  sinkCmd(l.firstWhereOrNull((e) => true)!); // @sink(none)
}

void decoy() { // @decoy
  final t = src('dart-collection-where-or-null-decoy'); // @source(dart-collection-where-or-null-decoy)
  final l = <String>['constant'];
  l.firstWhereOrNull((e) {
    sinkCmd(e); // @sink(none) @nosink(dart-collection-where-or-null-decoy)
    return true;
  });
  l.lastWhereOrNull((e) {
    sinkCmd(e); // @sink(none) @nosink(dart-collection-where-or-null-decoy)
    return true;
  });
  l.singleWhereOrNull((e) {
    sinkCmd(e); // @sink(none) @nosink(dart-collection-where-or-null-decoy)
    return true;
  });
  l.whereNot((e) {
    sinkCmd(e); // @sink(none) @nosink(dart-collection-where-or-null-decoy)
    return false;
  }).toList();
  l.none((e) {
    sinkCmd(e); // @sink(none) @nosink(dart-collection-where-or-null-decoy)
    return false;
  });
  sinkSql(t); // @sink(dart-collection-where-or-null-decoy)
}
