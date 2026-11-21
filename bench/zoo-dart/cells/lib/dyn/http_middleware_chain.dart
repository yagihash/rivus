// @cell(F/dyn/http-middleware-chain)
// @shape: handlers are chained (the outer one calls next.handle); the value is a field of the inner handler
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dyn/http-middleware-chain/vuln', vuln);
  zooCase('F/dyn/http-middleware-chain/fixed', fixed);
  zooCase('F/dyn/http-middleware-chain/decoy', decoy);
}

abstract class Handler {
  void handle(String path);
}

void serve(Handler h) {
  h.handle('/');
}

class Logging implements Handler {
  Logging(this.next);
  final Handler next;

  @override
  void handle(String path) {
    next.handle(path);
  }
}

class Inner implements Handler {
  Inner(this.v);
  final String v;

  @override
  void handle(String path) {
    sinkCmd(v); // @sink(dyn-http-middleware-chain-vuln) @noisy(1) @ref(DYN-IMPLS)
  }
}

void vuln() { // @vuln
  serve(Logging(Inner(src('dyn-http-middleware-chain-vuln')))); // @source(dyn-http-middleware-chain-vuln)
}

class LoggingFixed implements Handler {
  LoggingFixed(this.next);
  final Handler next;

  @override
  void handle(String path) {
    next.handle(path);
  }
}

class InnerFixed implements Handler {
  InnerFixed(this.v);
  final String v;

  @override
  void handle(String path) {
    sinkCmd(v); // @sink(none) @fp @ref(XTALK)
  }
}

void fixed() { // @fixed
  serve(LoggingFixed(InnerFixed(cleanCmd(src('dyn-http-middleware-chain-fixed')))));
}

class LoggingDecoy implements Handler {
  LoggingDecoy(this.next);
  final Handler next;

  @override
  void handle(String path) {
    next.handle(path);
  }
}

class InnerDecoy implements Handler {
  InnerDecoy(this.a, this.b);
  final String a;
  final String b;

  @override
  void handle(String path) {
    sinkCmd(b); // @sink(none) @fp @nosink(dyn-http-middleware-chain-decoy) @fp @ref(XTALK)
    sinkSql(a); // @sink(dyn-http-middleware-chain-decoy) @noisy(1) @ref(DYN-IMPLS)
  }
}

void decoy() { // @decoy
  serve(LoggingDecoy(InnerDecoy(src('dyn-http-middleware-chain-decoy'), 'constant'))); // @source(dyn-http-middleware-chain-decoy)
}
