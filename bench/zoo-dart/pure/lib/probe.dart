// Calls of the core library on receivers of the static types that the built-in model has rows for, so that the
// extractor shows which name each call gets (a call on a List is written as the member of Iterable that List
// inherits, a call on a File as the one of FileSystemEntity when File does not declare it). TestBuiltinRowsAreHitByTheZoo
// reads the IR of this package: a row of the model has to be spelled the way the extractor spells it.
// Nothing here runs (rivus reads it; TestZooPathDart runs observe.dart only).
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

void probeCollections(List<String> l, Set<String> s, StringBuffer sb) {
  print(l.first);
  print(l.last);
  print(l.single);
  print(l.join(','));
  print(l.toList());
  print(l.toSet());
  print(l.isEmpty);
  print(l.isNotEmpty);
  print(l.reversed);
  l.forEach((x) {});
  print(s.first);
  print(s.last);
  print(s.single);
  print(s.join(','));
  print(s.toList());
  s.forEach((x) {});
  print(sb.isEmpty);
  print(sb.isNotEmpty);
  sb.clear();
  print(Iterable<int>.generate(3));
  print(List<int>.generate(3, (i) => i));
}

void probeIo(File f, Directory d, Link k, IOSink o, HttpResponse r, HttpHeaders h, Cookie c, HeaderValue hv) {
  print(f.path);
  print(d.path);
  print(f.resolveSymbolicLinksSync());
  print(d.resolveSymbolicLinksSync());
  print(k.resolveSymbolicLinksSync());
  o.write('x');
  o.writeln('x');
  o.close();
  r.write('x');
  r.writeln('x');
  r.close();
  print(h.value('a'));
  print(c.value);
  print(hv.value);
}

Future<void> probeAsync(Stream<String> st, StreamSink<String> sink, EventSink<String> ev, StreamIterator<String> it, Completer<String> c) async {
  Stream<String>.error('x');
  st.timeout(const Duration(seconds: 1));
  await st.forEach((x) {});
  sink.addError('x');
  ev.addError('x');
  await sink.close();
  print(it.current);
  await it.cancel();
  Completer<String>.sync();
  print(c.future);
}

void probeConvert(Base64Codec b, Converter<String, String> cv, HtmlEscape he) {
  print(b.encode([1]));
  print(b.decode('x'));
  print(cv.convert('x'));
  print(he.convert('x'));
  print(num.parse('1'));
  print(RangeError.value(1));
  print(ArgumentError.value(1));
}

void probeTyped(Uint8List u, ByteData bd) {
  print(Uint8List.view(u.buffer));
  print(Uint32List.fromList([1]));
  print(Uint8List.fromList([1]));
  print(ByteData.sublistView(u));
  print(Uint8List.sublistView(bd));
  print(ByteData.view(u.buffer));
}
