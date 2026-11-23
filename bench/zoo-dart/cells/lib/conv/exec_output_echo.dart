// @cell(F/conv/exec-output-echo)
// @shape: a program prints its argument (echo), and what the program wrote to its output is the sink; Process.runSync takes the argument in the call, so the call itself is a command_injection sink of it too (Go sets cmd.Args after the constructor and has no such sink)
import 'dart:io';

import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/exec-output-echo/vuln', vuln);
  zooCase('F/conv/exec-output-echo/fixed', fixed);
  zooCase('F/conv/exec-output-echo/decoy', decoy);
}

void vuln() { // @vuln
  final r = Process.runSync('echo', [src('conv-exec-output-echo-vuln')]); // @source(conv-exec-output-echo-vuln) @noisy(1) @ref(PROCESS-ARGS)
  sinkHtml(r.stdout as String); // @sink(conv-exec-output-echo-vuln)
}

void fixed() { // @fixed
  final r = Process.runSync('echo', [cleanCmd(cleanHtml(src('conv-exec-output-echo-fixed')))]);
  sinkHtml(r.stdout as String); // @sink(none)
}

void decoy() { // @decoy
  final a = Process.runSync('echo', [src('conv-exec-output-echo-decoy')]); // @source(conv-exec-output-echo-decoy) @noisy(1) @ref(PROCESS-ARGS)
  final b = Process.runSync('echo', ['constant']);
  sinkHtml(b.stdout as String); // @sink(none) @nosink(conv-exec-output-echo-decoy)
  sinkSql(a.stdout as String); // @sink(conv-exec-output-echo-decoy)
}
