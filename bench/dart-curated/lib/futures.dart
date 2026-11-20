import 'lib.dart';

// Futures and their callbacks.

Future<String> fetch() async => source(); // @source(then-arg)

void then_value() {
  fetch().then((v) => sink(v)); // @sink(then-arg)
}

void then_chain() {
  final f = Future.value(source()); // @source(then-chain)
  f.then((v) => v.trim()).then((w) => sink(w)); // @sink(then-chain)
}

void then_return() {
  final f = Future.value(source()); // @source(then-ret)
  final g = f.then((v) => v + '!');
  g.then((w) => sink(w)); // @sink(then-ret)
}

void then_const() {
  final f = Future.value(source()); // @source(then-const)
  final g = f.then((v) => 'fixed');
  g.then((w) => sink(w)); // @sink(none) @fp
  f.then((v) => sink(v)); // @sink(then-const)
}

Future<void> await_then() async {
  final v = await Future.value(source()).then((x) => x); // @source(await-then)
  sink(v); // @sink(await-then)
}

void catch_error_value() {
  Future<String>.error(source()) // @source(catch-error) @miss
      .catchError((e) {
    sink(e); // @sink(catch-error)
    return '';
  });
}

void when_complete_capture() {
  String? out;
  final t = source(); // @source(when-complete) @miss
  Future.value(1).whenComplete(() {
    out = t;
  });
  sink(out); // @sink(when-complete)
}

Future<void> async_closure() async {
  final t = source(); // @source(async-closure)
  final f = () async => t;
  sink(await f()); // @sink(async-closure)
}

void completer_flow() {
  final t = source(); // @source(completer) @miss
  final c = Future<String>.sync(() => t);
  c.then((v) => sink(v)); // @sink(completer)
}

void stream_listen() {
  final s = Stream.value(source()); // @source(stream-listen)
  s.listen((v) => sink(v)); // @sink(stream-listen)
}

void stream_map() {
  final s = Stream.value(source()); // @source(stream-map)
  s.map((v) => v.trim()).listen((w) => sink(w)); // @sink(stream-map)
}
