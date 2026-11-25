// Calls the functions of the core library whose built-in model has a row of calls (the function calls a function
// it is given) with a closure that records what it was called with, and writes what came out as JSON.
// cmd/rivus TestZooCallbacksDart holds the rows to it (the same check as Go and Rust, zoo_callbacks_test.go): the
// answer is the run, not the model.
//
// The receiver has the marker RECV; what the closures return is the marker "Ж".
import 'dart:convert';

class Rec {
  int calls = 0;
  List<String> params = [];
  void see(List<Object?> p) {
    if (calls == 0) params = p.map((e) => '$e').toList();
    calls++;
  }
}

final out = <Map<String, Object?>>[];

void add(String symbol, Map<String, String> markers, Rec r, Object? result) {
  out.add({'symbol': symbol, 'markers': markers, 'calls': r.calls, 'params': r.params, 'result': '$result'});
}

const recv = {'recv': 'RECV'};

Future<void> main() async {
  {
    final r = Rec();
    final res = <String>['RECV'].map((x) { r.see([x]); return 'Ж'; }).toList();
    add('dart:core#List.map', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.map((x) { r.see([x]); return 'Ж'; }).toList();
    add('dart:core#Iterable.map', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.where((x) { r.see([x]); return true; }).toList();
    add('dart:core#Iterable.where', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.firstWhere((x) { r.see([x]); return true; });
    add('dart:core#Iterable.firstWhere', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.singleWhere((x) { r.see([x]); return true; });
    add('dart:core#Iterable.singleWhere', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.expand((x) { r.see([x]); return <String>['Ж']; }).toList();
    add('dart:core#Iterable.expand', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.lastWhere((x) { r.see([x]); return true; });
    add('dart:core#Iterable.lastWhere', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.any((x) { r.see([x]); return false; });
    add('dart:core#Iterable.any', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.every((x) { r.see([x]); return true; });
    add('dart:core#Iterable.every', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    it.forEach((x) { r.see([x]); });
    add('dart:core#Iterable.forEach', recv, r, '');
  }
  {
    final r = Rec();
    final l = <String>['RECV'];
    l.removeWhere((x) { r.see([x]); return false; });
    add('dart:core#List.removeWhere', recv, r, l);
  }
  {
    final r = Rec();
    final l = <String>['RECV'];
    l.retainWhere((x) { r.see([x]); return true; });
    add('dart:core#List.retainWhere', recv, r, l);
  }
  {
    final r = Rec();
    final s = <String>{'RECV'};
    s.removeWhere((x) { r.see([x]); return false; });
    add('dart:core#Set.removeWhere', recv, r, s);
  }
  {
    final r = Rec();
    final s = <String>{'RECV'};
    s.retainWhere((x) { r.see([x]); return true; });
    add('dart:core#Set.retainWhere', recv, r, s);
  }
  {
    final r = Rec();
    final res = <String>['RECV'].indexWhere((x) { r.see([x]); return true; });
    add('dart:core#List.indexWhere', recv, r, res);
  }
  {
    final r = Rec();
    final res = <String>['RECV'].lastIndexWhere((x) { r.see([x]); return true; });
    add('dart:core#List.lastIndexWhere', recv, r, res);
  }
  {
    final r = Rec();
    final m = <String, String>{'RECV': 'RECV'};
    m.removeWhere((k, v) { r.see([k, v]); return false; });
    add('dart:core#Map.removeWhere', recv, r, m);
  }
  {
    final r = Rec();
    final res = <String, String>{'RECV': 'RECV'}.map((k, v) { r.see([k, v]); return MapEntry('Ж', 'Ж'); });
    add('dart:core#Map.map', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.takeWhile((x) { r.see([x]); return true; }).toList();
    add('dart:core#Iterable.takeWhile', recv, r, res);
  }
  {
    final r = Rec();
    final Iterable<String> it = Iterable<String>.generate(1, (_) => 'RECV');
    final res = it.skipWhile((x) { r.see([x]); return false; }).toList();
    add('dart:core#Iterable.skipWhile', recv, r, res);
  }
  {
    final r = Rec();
    final l = <String>['RECV', 'RECV'];
    l.sort((a, b) { r.see([a, b]); return 0; });
    add('dart:core#List.sort', recv, r, l);
  }
  {
    final r = Rec();
    final res = <String>['RECV', 'RECV'].reduce((a, b) { r.see([a, b]); return 'Ж'; });
    add('dart:core#Iterable.reduce', recv, r, res);
  }
  {
    final r = Rec();
    final res = <String>['RECV'].fold<String>('INIT', (a, b) { r.see([a, b]); return 'Ж'; });
    add('dart:core#Iterable.fold', {'arg0': 'INIT', 'recv': 'RECV'}, r, res);
  }
  // a forEach on a List or a Set is written by the extractor as Iterable.forEach (they inherit it), so it is the one row
  {
    final r = Rec();
    <String, String>{'RECV': 'RECV'}.forEach((k, v) { r.see([k, v]); });
    add('dart:core#Map.forEach', recv, r, '');
  }
  {
    final r = Rec();
    final res = await Future<String>.value('RECV').then((x) { r.see([x]); return 'Ж'; });
    add('dart:async#Future.then', recv, r, res);
  }
  {
    final r = Rec();
    final res = await Future<String>.error('RECV').catchError((Object e) { r.see([e]); return 'Ж'; });
    add('dart:async#Future.catchError', recv, r, res);
  }
  {
    final r = Rec();
    final res = await Future<String>.value('RECV').whenComplete(() { r.see([]); return 'Ж'; });
    add('dart:async#Future.whenComplete', recv, r, res);
  }
  {
    final r = Rec();
    final res = await Stream<String>.value('RECV').map((x) { r.see([x]); return 'Ж'; }).toList();
    add('dart:async#Stream.map', recv, r, res);
  }
  {
    final r = Rec();
    final res = await Stream<String>.value('RECV').asyncMap((x) async { r.see([x]); return 'Ж'; }).toList();
    add('dart:async#Stream.asyncMap', recv, r, res);
  }
  {
    final r = Rec();
    final res = await Stream<String>.value('RECV').where((x) { r.see([x]); return true; }).toList();
    add('dart:async#Stream.where', recv, r, res);
  }
  {
    final r = Rec();
    final sub = Stream<String>.value('RECV').listen((x) { r.see([x]); });
    await sub.asFuture<void>();
    add('dart:async#Stream.listen', recv, r, '');
  }
  {
    final r = Rec();
    await Stream<String>.value('RECV').forEach((x) { r.see([x]); });
    add('dart:async#Stream.forEach', recv, r, '');
  }
  print(jsonEncode(out));
}
