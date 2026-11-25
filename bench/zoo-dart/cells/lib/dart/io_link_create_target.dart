// @cell(F/dart/io-link-create-target)
// @shape: the target of a symbolic link (Link.create, Link.createSync of dart:io) is a path sink of the built-in rows: a link whose target is a stored name points anywhere (zip slip by a link). The path of the link itself is Link.new. dart:io has no recording skeleton, so the same value is also given to sinkPath on the line: the run sees the sink there
import 'dart:io';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/io-link-create-target/vuln', vuln);
  zooCase('F/dart/io-link-create-target/fixed', fixed);
  zooCase('F/dart/io-link-create-target/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final tmp = Directory.systemTemp.createTempSync('zoo-link');
  try {
    final a = src('dart-io-link-create-target-vuln-sync'); // @source(dart-io-link-create-target-vuln-sync)
    sinkPath('/srv/base', a); Link('${tmp.path}/a').createSync(a); // @sink(dart-io-link-create-target-vuln-sync)
    final b = src('dart-io-link-create-target-vuln-async'); // @source(dart-io-link-create-target-vuln-async)
    sinkPath('/srv/base', b); await Link('${tmp.path}/b').create(b); // @sink(dart-io-link-create-target-vuln-async)
  } finally {
    tmp.deleteSync(recursive: true);
  }
}

Future<void> fixed() async { // @fixed
  final tmp = Directory.systemTemp.createTempSync('zoo-link');
  try {
    final a = cleanPath(src('dart-io-link-create-target-fixed-sync'));
    sinkPath('/srv/base', a); Link('${tmp.path}/a').createSync(a); // @sink(none)
    final b = cleanPath(src('dart-io-link-create-target-fixed-async'));
    sinkPath('/srv/base', b); await Link('${tmp.path}/b').create(b); // @sink(none)
  } finally {
    tmp.deleteSync(recursive: true);
  }
}

Future<void> decoy() async { // @decoy
  final tmp = Directory.systemTemp.createTempSync('zoo-link');
  try {
    final v = src('dart-io-link-create-target-decoy'); // @source(dart-io-link-create-target-decoy)
    sinkPath('/srv/base', 'constant'); Link('${tmp.path}/a').createSync('constant'); // @sink(none) @nosink(dart-io-link-create-target-decoy)
    sinkSql(v); // @sink(dart-io-link-create-target-decoy)
  } finally {
    tmp.deleteSync(recursive: true);
  }
}
