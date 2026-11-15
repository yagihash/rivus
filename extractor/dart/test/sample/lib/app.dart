import 'lib.dart';

class Box {
  Box(this.value);
  String value;
}

String direct() {
  final s = source(); // @source(direct)
  sink(s); // @sink(direct)
  return s;
}

void concat(String id) {
  final q = 'SELECT * FROM t WHERE id = ' + id;
  sink(q);
}

void interpolated() {
  final s = source(); // @source(interp)
  sink('SELECT $s FROM t'); // @sink(interp)
}

void viaField() {
  final b = Box(source()); // @source(field)
  sink(b.value); // @sink(field)
}

void viaFieldWrite() {
  final b = Box('');
  b.value = source(); // @source(fieldwrite)
  sink(b.value); // @sink(fieldwrite)
}

void branches(bool c) {
  var s = 'safe';
  if (c) {
    s = source(); // @source(branch)
  }
  sink(s); // @sink(branch)
}

void literalOnly() {
  sink('constant'); // @sink(none)
}

void loop(List<String> xs) {
  for (final x in xs) {
    sink(x); // @sink(none)
  }
}

void viaList() {
  final xs = [source()]; // @source(list)
  sink(xs[0]); // @sink(list)
}

void viaClosure() {
  final s = source(); // @source(closure)
  void emit() {
    sink(s); // @sink(closure)
  }

  emit();
}

void viaMutatedClosure() {
  var s = 'safe';
  void emit() {
    sink(s); // @sink(mutated)
  }

  s = source(); // @source(mutated)
  emit();
}

String sanitized() {
  final s = source();
  final t = sanitize(s);
  sink(t); // @sink(none)
  return t;
}

Future<void> async1() async {
  final s = await fetch();
  sink(s); // @sink(await)
}

Future<String> fetch() async => source(); // @source(await)

void tryCatch() {
  try {
    final s = source(); // @source(try)
    sink(s); // @sink(try)
  } catch (e) {
    sink(e.toString()); // @sink(none)
  } finally {
    sink('done'); // @sink(none)
  }
}

void viaCollectionFor() {
  final xs = [source()]; // @source(for-elem)
  final ys = [for (final x in xs) x.trim()];
  sink(ys.first); // @sink(for-elem)
}

void viaIfElement(bool c) {
  final ys = [if (c) source() else 'safe']; // @source(if-elem)
  sink(ys.first); // @sink(if-elem)
}
