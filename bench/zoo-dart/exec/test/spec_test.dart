// Holds the constants of zoo_mark to bench/zoo/spec.json and runs its probes (in the variant of ZOO_VARIANT).
import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';
import 'package:zoo_mark/zoo_mark.dart' as mark;

void main() {
  final spec = jsonDecode(File('../../zoo/spec.json').readAsStringSync()) as Map<String, Object?>;
  final classes = spec['classes']! as Map<String, Object?>;

  test('the constants are the spec', () {
    expect(spec['prefix'], 'Z');
    expect(spec['order'], mark.order);
    for (final c in mark.order) {
      final s = classes[c]! as Map<String, Object?>;
      expect(mark.specs[c]!.segment, s['segment'], reason: c);
      expect(mark.specs[c]!.danger, s['danger'], reason: c);
      expect(mark.specs[c]!.set, s['set'], reason: c);
      final d = (s['danger']! as List).cast<String>();
      expect(d[1].length, d[0].length, reason: '$c: the dangers differ in length');
      for (final danger in d) {
        for (final r in danger.split('')) {
          expect((s['set']! as String).contains(r), isTrue, reason: '$c: $r of the danger is not in the set');
        }
      }
    }
  });

  test('the set of a class holds no character of the danger of another class', () {
    for (final c in mark.order) {
      for (final o in mark.order) {
        if (o == c) continue;
        for (final r in mark.specs[c]!.set.split('')) {
          expect(mark.specs[o]!.danger.join().contains(r), isFalse, reason: 'the set of $c has $r, which is in the danger of $o');
        }
      }
    }
  });

  test('the probes', () {
    for (final p in (spec['probes']! as List).cast<Map<String, Object?>>()) {
      final cls = p['class']! as String;
      final d = mark.danger(cls);
      final text = (p['text']! as String).replaceAll('{DQ}', d[0] + d).replaceAll('{D}', d);
      final got = mark.judge(cls, (p['base'] as String?) ?? '', text);
      expect(got, p['reached'], reason: 'probe $cls "$text" (variant ${mark.variant})');
    }
  });

  test('a sanitizer gives the same text for both variants', () {
    final clean = {
      mark.classCmd: mark.cleanCmd,
      mark.classSql: mark.cleanSql,
      mark.classHtml: mark.cleanHtml,
      mark.classPath: mark.cleanPath,
    };
    for (final c in mark.order) {
      final d = mark.specs[c]!.danger;
      expect(clean[c]!(d[0]), clean[c]!(d[1]), reason: c);
      expect(clean[c]!(d[0]).replaceAll('_', ''), isEmpty, reason: '$c: a danger is left');
    }
  });
}
