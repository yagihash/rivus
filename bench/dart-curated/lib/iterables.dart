import 'lib.dart';

// The callbacks of the core library.

void map_to_list() {
  final items = [source()]; // @source(map-list)
  final out = items.map((s) => s.trim()).toList();
  sink(out.first); // @sink(map-list)
}

void map_other_list() {
  final items = [source()]; // @source(map-other)
  final other = ['const'];
  final out = other.map((s) => s.trim()).toList();
  sink(out.first); // @sink(none)
  sink(items); // @sink(map-other)
}

void for_each_sink() {
  final items = [source()]; // @source(foreach)
  items.forEach((s) => sink(s)); // @sink(foreach)
}

void for_each_capture() {
  String? last;
  final items = [source()]; // @source(foreach-cap)
  items.forEach((s) {
    last = s;
  });
  sink(last); // @sink(foreach-cap)
}

void where_keeps() {
  final items = [source()]; // @source(where)
  final out = items.where((s) => s.isNotEmpty).toList();
  sink(out.first); // @sink(where)
}

void fold_value() {
  final items = [source()]; // @source(fold)
  final out = items.fold<String>('', (acc, s) => acc + s);
  sink(out); // @sink(fold)
}

void reduce_value() {
  final items = [source(), 'b']; // @source(reduce)
  final out = items.reduce((a, b) => a + b);
  sink(out); // @sink(reduce)
}

void expand_value() {
  final items = [source()]; // @source(expand)
  final out = items.expand((s) => [s, s]).toList();
  sink(out.last); // @sink(expand)
}

void map_const_result() {
  final items = [source()]; // @source(map-const)
  final out = items.map((s) => 'fixed').toList();
  sink(out.first); // @sink(none)
  sink(items); // @sink(map-const)
}

void any_callback() {
  final items = [source()]; // @source(any)
  items.any((s) {
    sink(s); // @sink(any)
    return true;
  });
}

void sort_by() {
  final items = [source(), 'b']; // @source(sort)
  items.sort((a, b) => a.compareTo(b));
  sink(items.first); // @sink(sort)
}

void map_entries() {
  final m = {'k': source()}; // @source(map-entries)
  m.forEach((k, v) => sink(v)); // @sink(map-entries)
}

void putIfAbsent_value() {
  final m = <String, String>{};
  final t = source(); // @source(put-if-absent) @miss(assumed_call)
  final v = m.putIfAbsent('k', () => t);
  sink(v); // @sink(put-if-absent)
}
